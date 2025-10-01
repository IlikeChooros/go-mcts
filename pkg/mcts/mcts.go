package mcts

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"sync"
	"sync/atomic"
	"unsafe"
)

type TreeStats struct {
	// size     atomic.Int32
	maxdepth atomic.Int32
	cps      atomic.Uint32
	cycles   atomic.Uint32
}

type MCTS[T MoveLike, S NodeStatsLike, R GameResult] struct {
	TreeStats
	listener          *StatsListener[T]
	Limiter           LimiterLike
	selection_policy  SelectionPolicy[T, S]
	Root              *NodeBase[T, S]
	size              atomic.Uint32
	wg                sync.WaitGroup
	collisionCount    atomic.Int32
	multithreadPolicy MultithreadPolicy
	roots             []*NodeBase[T, S]
	merged            atomic.Bool
	strategy          StrategyLike[T, S, R]
}

// Create new base tree
func NewMTCS[T MoveLike, S NodeStatsLike, R GameResult](
	selectionPolicy SelectionPolicy[T, S],
	operations GameOperations[T, S, R],
	flags uint32,
	multithreadPolicy MultithreadPolicy,
	defaultStats S,
	strategy StrategyLike[T, S, R],
) *MCTS[T, S, R] {
	mcts := &MCTS[T, S, R]{
		TreeStats:         TreeStats{},
		listener:          &StatsListener[T]{},
		Limiter:           LimiterLike(NewLimiter(uint32(unsafe.Sizeof(NodeBaseDefault[T]{})))),
		selection_policy:  selectionPolicy,
		Root:              &NodeBase[T, S]{Flags: flags, Stats: defaultStats},
		multithreadPolicy: multithreadPolicy,
		strategy:          strategy,
	}

	// Set IsSearching to false
	mcts.Limiter.Stop()

	// If that's random-based playouts, attach random number generator
	if rg, ok := operations.(RandGameOperations[T, S, R]); ok {
		rg.SetRand(rand.New(rand.NewSource(SeedGeneratorFn())))
	}

	// Expand the root node, by default
	if mcts.Root.CanExpand() {
		mcts.Root.FinishExpanding()
		mcts.size.Store(1 + operations.ExpandNode(mcts.Root))
	} else {
		mcts.size.Store(1)
	}

	return mcts
}

func (mcts *MCTS[T, S, R]) invokeListener(f ListenerFunc[T]) {
	if f != nil {
		f(toListenerStats(mcts))
	}
}

// The number of times a node was chosen, but it was already being expanded.
// Resulting in a 'waiting' state of the search thread
func (mcts *MCTS[T, S, R]) CollisionCount() int32 {
	return mcts.collisionCount.Load()
}

// Number of all collisions in the tree divided by the number of all cycles,
// for more info see CollisionCount
func (mcts *MCTS[T, S, R]) CollisionFactor() float64 {
	return float64(mcts.collisionCount.Load()) / float64(mcts.Cycles())
}

func (mcts *MCTS[T, S, R]) ResetListener() {
	mcts.listener.OnCycle(nil).OnDepth(nil).OnStop(nil)
}

func (mcts *MCTS[T, S, R]) StatsListener() *StatsListener[T] {
	return mcts.listener
}

func (mcts *MCTS[T, S, R]) SetListener(listener StatsListener[T]) {
	*mcts.listener = listener
}

// Adds custom context to the limiter, enabling cancellation through it
//
// Example:
//
//	ctx, cancel := context.WithCancel(context.Background())
//
//	tree.SetContext(ctx)
//	go func() {
//	    time.Sleep(2 * time.Second)
//	    cancel() // Cancel the search after 2 seconds
//	}()
//
//	tree.Search()
func (mcts *MCTS[T, S, R]) SetContext(ctx context.Context) {
	mcts.Limiter.SetContext(ctx)
}

func (mcts *MCTS[T, S, R]) SetMultithreadPolicy(policy MultithreadPolicy) {
	mcts.multithreadPolicy = policy
}

func (mcts *MCTS[T, S, R]) IsSearching() bool {
	return !mcts.Limiter.Stop()
}

// Stop the search
func (mcts *MCTS[T, S, R]) Stop() {
	mcts.Limiter.SetStop(true)
}

// Maxiumum depth reach during the search, note that usually MaxDepth != len(pv)
func (mcts *MCTS[T, S, R]) MaxDepth() int {
	return int(mcts.maxdepth.Load())
}

// Total number of 'iterations', 'cycles', 'simluations' ran during the search
func (mcts *MCTS[T, S, R]) Cycles() int {
	return int(mcts.cycles.Load())
}

// Get cycles per second statistic
func (mcts *MCTS[T, S, R]) Cps() uint32 {
	return mcts.cps.Load()
}

// Get the reason why the search was stopped, valid after search ends
func (mcts *MCTS[T, S, R]) StopReason() StopReason {
	return mcts.Limiter.StopReason()
}

func (mcts *MCTS[T, S, R]) SetLimits(limits *Limits) {
	mcts.Limiter.SetLimits(limits)
}

func (mcts *MCTS[T, S, R]) Limits() *Limits {
	return mcts.Limiter.Limits()
}

func (mcts *MCTS[T, S, R]) Strategy() StrategyLike[T, S, R] {
	return mcts.strategy
}

func (mcts *MCTS[T, S, R]) String() string {
	str := fmt.Sprintf("MCTS={Size=%d, Stats:{maxdepth=%d, cps=%d, cycles=%d}, Stop=%v",
		mcts.Size(), mcts.MaxDepth(), mcts.Cps(), mcts.Cycles(), !mcts.IsSearching())
	str += fmt.Sprintf(", Root=%v, Root.Children=%v", mcts.Root, mcts.Root.Children)
	return str
}

// Helper function to count tree nodes
func countTreeNodes[T MoveLike, S NodeStatsLike](node *NodeBase[T, S]) int {
	nodes := 1
	for i := range node.Children {
		if len(node.Children[i].Children) > 0 {
			nodes += countTreeNodes(&node.Children[i])
		} else {
			nodes += 1
		}
	}

	return nodes
}

// Get the size of the tree (by counting)
func (mcts *MCTS[T, S, R]) Count() int {
	return countTreeNodes(mcts.Root)
}

// Get the size of the tree
func (mcts *MCTS[T, S, R]) Size() uint32 {
	// Count every node in the tree
	return mcts.size.Load()
}

// Returns approximation of memory usage of the tree structure
func (mcts *MCTS[T, S, R]) MemoryUsage() uint32 {
	return mcts.Size()*uint32(unsafe.Sizeof(NodeBase[T, S]{})) + uint32(unsafe.Sizeof(MCTS[T, S, R]{}))
}

// Tries to make given 'move' a new root, if it failes, does nothing
func (mcts *MCTS[T, S, R]) MakeMove(move T) {
	// If the search is running, stop it first
	if mcts.IsSearching() {
		mcts.Stop()
		mcts.Synchronize()
	}

	// Find the child with given move
	var newRoot *NodeBase[T, S]
	for i := range mcts.Root.Children {
		if mcts.Root.Children[i].Move == move {
			newRoot = &mcts.Root.Children[i]
			break
		}
	}

	if newRoot == nil {
		return
	}

	oldRoot := mcts.Root
	mcts.Root = newRoot
	mcts.size.Store(uint32(countTreeNodes(newRoot)))
	mcts.maxdepth.Store(max(0, int32(mcts.MaxDepth()-1)))

	// Detach the new root from its parent
	newRoot.Parent = nil

	// Clear the children of the old root, to make them available for GC
	oldRoot.Children = nil
}

// Remove previous tree & update game ops state
func (mcts *MCTS[T, S, R]) Reset(ops GameOperations[T, S, R], isTerminated bool, defaultStats S) {
	// Discard running search
	if mcts.IsSearching() {
		mcts.Stop()
		mcts.Synchronize()
	}

	// Reset game state and make new root
	ops.Reset()
	mcts.Root = newRootNode[T](isTerminated, defaultStats)
	mcts.size.Store(1)
	mcts.Root.CanExpand()
	mcts.Root.FinishExpanding()

	if !isTerminated {
		mcts.size.Add(ops.ExpandNode(mcts.Root))
	}
}

// 'the best move' in the position
func (mcts *MCTS[T, S, R]) RootMove() T {
	var signature T
	if bestChild := mcts.BestChild(mcts.Root, BestChildMostVisits); bestChild != nil {
		signature = bestChild.Move
	}
	return signature
}

// Current evaluation of the position
func (mcts *MCTS[T, S, R]) RootScore() Result {
	if bestChild := mcts.BestChild(mcts.Root, BestChildMostVisits); bestChild != nil {
		return bestChild.Stats.Outcomes() / Result(bestChild.Stats.Visits())
	}
	return Result(math.NaN())
}

// Return best child, based on the policy
func (mcts *MCTS[T, S, R]) BestChild(node *NodeBase[T, S], policy BestChildPolicy) *NodeBase[T, S] {
	var bestChild *NodeBase[T, S]
	var child *NodeBase[T, S]
	maxVisits := 0

	// DEBUG
	// rootTurn := mcts.Root.Turn() == node.Turn()
	// if rootTurn {
	// 	fmt.Print("Root's turn")
	// } else {
	// 	fmt.Print("Enemy's turn")
	// }
	// fmt.Printf(" wr=%0.2f\n", float64(node.Stats.Outcomes())/float64(node.Stats.Visits()))
	// for i := range node.Children {
	// 	ch := &node.Children[i]
	// 	fmt.Printf("%d. %v v=%d (wr=%.2f)\n",
	// 		i+1, ch.Move, ch.Stats.Visits(),
	// 		float64(ch.Stats.Outcomes())/float64(ch.Stats.Visits()),
	// 	)
	// }

	switch policy {
	case BestChildMostVisits:
		for i := 0; i < len(node.Children); i++ {
			child = &node.Children[i]
			if v := int(child.Stats.RealVisits()); v > maxVisits && v > 0 {
				maxVisits = int(child.Stats.RealVisits())
				bestChild = child
			}
		}
	case BestChildWinRate:
		// the child we choose should have at least 20% of the max visit count (from the neighbours)
		const (
			minVisitsPercentageThreshold = 0
			minVisitsThreshold           = 10
		)

		bestWinRate := -1.0

		// Get max visits out the children
		for i := 0; i < len(node.Children); i++ {
			maxVisits = max(int(node.Children[i].Stats.Visits()), maxVisits)
		}

		// Go through the children
		for i := 0; i < len(node.Children); i++ {
			child = &node.Children[i]
			real := child.Stats.RealVisits()
			if real > minVisitsThreshold && real > int32(minVisitsPercentageThreshold*float64(maxVisits)) {

				// We optimize the winning chances, looking from the root's perspective
				var winRate float64 = float64(child.Stats.Outcomes()) / float64(child.Stats.Visits())

				if winRate > bestWinRate {
					bestWinRate = winRate
					bestChild = child
				}
			}
		}
	}

	// if bestChild != nil {
	// 	fmt.Println("Chose", bestChild.Move)
	// }

	return bestChild
}

type PvResult[T MoveLike, S NodeStatsLike] struct {
	Root     *NodeBase[T, S]
	Pv       []T
	Terminal bool
	Draw     bool
}

// Returns 'pvCount' best move lines, specified in the limits
func (mcts *MCTS[T, S, R]) MultiPv(policy BestChildPolicy) []PvResult[T, S] {
	if mcts.Root == nil {
		return nil
	}

	pvCount := mcts.Limiter.Limits().MultiPv
	multipv := make([]PvResult[T, S], 0, pvCount)
	child_count := len(mcts.Root.Children)
	root_nodes := make([]*NodeBase[T, S], child_count)
	for i := range child_count {
		root_nodes[i] = &mcts.Root.Children[i]
	}

	slices.SortFunc(root_nodes, func(a *NodeBase[T, S], b *NodeBase[T, S]) int {
		va, vb := a.Stats.Visits(), b.Stats.Visits()
		if va < vb {
			return 1
		} else if va > vb {
			return -1
		}
		return 0
	})

	for i := range pvCount {
		// Get the Pv from this 'Root'
		if i < child_count {
			pv, terminal, draw := mcts.Pv(root_nodes[i], policy, true)
			multipv = append(multipv, PvResult[T, S]{
				Root:     root_nodes[i],
				Pv:       pv,
				Terminal: terminal,
				Draw:     draw,
			})
		} else {
			break
		}
	}

	return multipv
}

// Get the principal variation (ie. the best sequence of moves)
// from given starting 'root' node, based on given best child policy
func (mcts *MCTS[T, S, R]) PvNodes(root *NodeBase[T, S], policy BestChildPolicy, includeRoot bool) ([]*NodeBase[T, S], bool) {
	if root == nil {
		return nil, false
	}

	pv := make([]*NodeBase[T, S], 0, mcts.MaxDepth()+1)
	node := root
	mate := false

	if includeRoot {
		pv = append(pv, root)
	}

	if len(root.Children) == 0 {
		// If there are no children, we cannot go further
		return pv, root.Terminal()
	}

	// Simply select 'best child' until we don't have any children
	// or the node is nil
	for len(node.Children) > 0 {
		node = mcts.BestChild(node, policy)
		if node == nil {
			break
		}

		pv = append(pv, node)

		// If that's a terminal node, we got a mate score
		if node.Terminal() {
			mate = true
			break
		}
	}

	return pv, mate
}

// Get the pricipal variation, but only the moves, returns (moves, mate, draw)
func (mcts *MCTS[T, S, R]) Pv(root *NodeBase[T, S], policy BestChildPolicy, includeRoot bool) ([]T, bool, bool) {
	if root == nil {
		return nil, false, false
	}

	var node *NodeBase[T, S]
	nodes, mate := mcts.PvNodes(root, policy, includeRoot)
	pv := make([]T, len(nodes))
	for i := range len(nodes) {
		node = nodes[i]
		pv[i] = node.Move
	}

	return pv, mate, (mate && node.Stats.AvgOutcome() == 0.5)
}
