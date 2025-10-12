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

type MCTSLike[T MoveLike, S NodeStatsLike[S], R GameResult, O GameOperations[T, S, R, O], A StrategyLike[T, S, R, O]] interface {
	// Get the size of the tree
	Size() uint32
	// Get the size of the tree (by counting)
	Count() int
	// Returns approximation of memory usage of the tree structure
	MemoryUsage() uint32
	// Get cycles per second statistic
	Cps() uint32
	// Get the reason why the search was stopped, valid after search ends
	StopReason() StopReason
	// Maxiumum depth reach during the search, note that usually MaxDepth != len(pv)
	MaxDepth() int
	// Total number of 'iterations', 'cycles', 'simluations' ran during the search
	Cycles() int
	// The number of times a node was chosen, but it was already being expanded.
	// Resulting in a 'waiting' state of the search thread
	CollisionCount() int32
	// Number of all collisions in the tree divided by the number of all cycles,
	// for more info see CollisionCount
	CollisionFactor() float64
	// Is the search currently running
	IsRunning() bool
	// Stop the search
	Stop()
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
	SetContext(ctx context.Context)
	// Set search limits
	SetLimits(limits *Limits)
	// Get current search limits
	Limits() *Limits
	// Get the strategy used by this MCTS instance
	Strategy() A
	// Reset the listener functions
	ResetListener()
	// Get the stats listener
	StatsListener() *StatsListener[T]
	// Set a custom stats listener
	SetListener(listener StatsListener[T])
	// Set multithreading policy
	SetMultithreadPolicy(policy MultithreadPolicy)
	// Tries to make given 'move' a new root, if it failes, does nothing
	MakeMove(move T)
	// 'the best move' in the position
	BestMove() T
	// Current evaluation of the position
	RootScore() Result
	// Return best child, based on the policy
	BestChild(node *NodeBase[T, S], policy BestChildPolicy) *NodeBase[T, S]
	// Get the principal variation (ie. the best sequence of moves)
	// from given starting 'root' node, based on given best child policy
	PvNodes(root *NodeBase[T, S], policy BestChildPolicy, includeRoot bool) ([]*NodeBase[T, S], bool)
	// Get the pricipal variation, but only the moves, returns (moves, mate, draw)
	Pv(root *NodeBase[T, S], policy BestChildPolicy, includeRoot bool) ([]T, bool, bool)
	// Returns 'pvCount' best move lines, specified in the limits
	MultiPv(policy BestChildPolicy) []PvResult[T, S]
	// Reset the tree & update game ops state
	Reset(isTerminated bool)
}

type MCTS[T MoveLike, S NodeStatsLike[S], R GameResult, O GameOperations[T, S, R, O], A StrategyLike[T, S, R, O]] struct {
	TreeStats
	listener          *StatsListener[T]
	Limiter           LimiterLike
	Root              *NodeBase[T, S]
	size              atomic.Uint32
	wg                sync.WaitGroup
	collisionCount    atomic.Int32
	multithreadPolicy MultithreadPolicy
	roots             []*NodeBase[T, S]
	merged            atomic.Bool
	strategy          A
	ops               O
}

// Create new base tree
func NewMTCS[T MoveLike, S NodeStatsLike[S], R GameResult, O GameOperations[T, S, R, O], A StrategyLike[T, S, R, O]](
	strategy A,
	operations O,
	multithreadPolicy MultithreadPolicy,
	defaultStats S,
) *MCTS[T, S, R, O, A] {
	mcts := &MCTS[T, S, R, O, A]{
		TreeStats:         TreeStats{},
		listener:          &StatsListener[T]{},
		Limiter:           NewLimiter(uint32(unsafe.Sizeof(NodeBase[T, S]{}))),
		Root:              &NodeBase[T, S]{Flags: 0, Stats: defaultStats},
		multithreadPolicy: multithreadPolicy,
		strategy:          strategy,
		ops:               operations,
	}

	// Set IsSearching to false
	mcts.Limiter.Stop()

	// If that's random-based playouts, attach random number generator
	if rg, ok := GameOperations[T, S, R, O](operations).(RandGameOperations[T, S, R, O]); ok {
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

func (mcts *MCTS[T, S, R, O, A]) invokeListener(f ListenerFunc[T]) {
	if f != nil {
		f(toListenerStats(mcts))
	}
}

// The number of times a node was chosen, but it was already being expanded.
// Resulting in a 'waiting' state of the search thread
func (mcts *MCTS[T, S, R, O, A]) CollisionCount() int32 {
	return mcts.collisionCount.Load()
}

// Number of all collisions in the tree divided by the number of all cycles,
// for more info see CollisionCount
func (mcts *MCTS[T, S, R, O, A]) CollisionFactor() float64 {
	return float64(mcts.collisionCount.Load()) / float64(mcts.Cycles())
}

func (mcts *MCTS[T, S, R, O, A]) ResetListener() {
	mcts.listener.OnCycle(nil).OnDepth(nil).OnStop(nil)
}

func (mcts *MCTS[T, S, R, O, A]) StatsListener() *StatsListener[T] {
	return mcts.listener
}

func (mcts *MCTS[T, S, R, O, A]) SetListener(listener StatsListener[T]) {
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
func (mcts *MCTS[T, S, R, O, A]) SetContext(ctx context.Context) {
	mcts.Limiter.SetContext(ctx)
}

func (mcts *MCTS[T, S, R, O, A]) MultithreadPolicy() MultithreadPolicy {
	return mcts.multithreadPolicy
}

func (mcts *MCTS[T, S, R, O, A]) SetMultithreadPolicy(policy MultithreadPolicy) {
	mcts.multithreadPolicy = policy
}

func (mcts *MCTS[T, S, R, O, A]) IsSearching() bool {
	return !mcts.Limiter.Stop()
}

// Stop the search
func (mcts *MCTS[T, S, R, O, A]) Stop() {
	mcts.Limiter.SetStop(true)
}

// Maxiumum depth reach during the search, note that usually MaxDepth != len(pv)
func (mcts *MCTS[T, S, R, O, A]) MaxDepth() int {
	return int(mcts.maxdepth.Load())
}

// Total number of 'iterations', 'cycles', 'simluations' ran during the search
func (mcts *MCTS[T, S, R, O, A]) Cycles() int {
	return int(mcts.cycles.Load())
}

// Get cycles per second statistic
func (mcts *MCTS[T, S, R, O, A]) Cps() uint32 {
	return mcts.cps.Load()
}

// Get the reason why the search was stopped, valid after search ends
func (mcts *MCTS[T, S, R, O, A]) StopReason() StopReason {
	return mcts.Limiter.StopReason()
}

func (mcts *MCTS[T, S, R, O, A]) SetLimits(limits *Limits) {
	mcts.Limiter.SetLimits(limits)
}

func (mcts *MCTS[T, S, R, O, A]) Limits() *Limits {
	return mcts.Limiter.Limits()
}

// Returns underlying selection and backpropagation strategy (UCB1, RAVE, etc)
func (mcts *MCTS[T, S, R, O, A]) Strategy() A {
	return mcts.strategy
}

func (mcts *MCTS[T, S, R, O, A]) String() string {
	str := fmt.Sprintf("MCTS={Size=%d, Stats:{maxdepth=%d, cps=%d, cycles=%d}, Stop=%v",
		mcts.Size(), mcts.MaxDepth(), mcts.Cps(), mcts.Cycles(), !mcts.IsSearching())
	str += fmt.Sprintf(", Root=%v, Root.Children=%v", mcts.Root, mcts.Root.Children)
	return str
}

// Helper function to count tree nodes
func countTreeNodes[T MoveLike, S NodeStatsLike[S]](node *NodeBase[T, S]) int {
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

func (mcts *MCTS[T, S, R, O, A]) Ops() O {
	return mcts.ops
}

// Get the size of the tree (by counting)
func (mcts *MCTS[T, S, R, O, A]) Count() int {
	return countTreeNodes(mcts.Root)
}

// Get the size of the tree
func (mcts *MCTS[T, S, R, O, A]) Size() uint32 {
	// Count every node in the tree
	return mcts.size.Load()
}

// Returns an approximation of memory usage of the tree structure
func (mcts *MCTS[T, S, R, O, A]) MemoryUsage() uint32 {
	return mcts.Size()*uint32(unsafe.Sizeof(NodeBase[T, S]{})) + uint32(unsafe.Sizeof(MCTS[T, S, R, O, A]{}))
}

// Creates a deep copy of the tree
func (mcts *MCTS[T, S, R, O, A]) Clone() *MCTS[T, S, R, O, A] {
	clone := &MCTS[T, S, R, O, A]{
		Root:              mcts.Root.Clone(nil),
		ops:               mcts.ops.Clone(),
		strategy:          mcts.strategy,
		multithreadPolicy: mcts.multithreadPolicy,
		listener:          &StatsListener[T]{},
		Limiter:           NewLimiter(uint32(unsafe.Sizeof(NodeBase[T, S]{}))),
	}

	clone.TreeStats.cps.Store(mcts.TreeStats.cps.Load())
	clone.TreeStats.cycles.Store(mcts.TreeStats.cycles.Load())
	clone.TreeStats.maxdepth.Store(mcts.TreeStats.maxdepth.Load())
	clone.collisionCount.Store(mcts.collisionCount.Load())
	clone.merged.Store(mcts.merged.Load())
	clone.size.Store(mcts.size.Load())

	return clone
}

// Tries to make given 'move' a new root, if it failes, does nothing
func (mcts *MCTS[T, S, R, O, A]) MakeMove(move T) bool {
	// If the search is running, stop it first
	if mcts.IsSearching() {
		mcts.Stop()
		mcts.Synchronize()
	}

	// Sanitity check
	if mcts.Root == nil || len(mcts.Root.Children) == 0 {
		return false
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
		return false
	}

	oldRoot := mcts.Root
	mcts.Root = newRoot
	mcts.size.Store(uint32(countTreeNodes(newRoot)))
	mcts.maxdepth.Store(max(0, int32(mcts.MaxDepth()-1)))
	mcts.ops.Traverse(move) // update game state

	// Detach the new root from its parent
	newRoot.Parent = nil

	// Clear the children of the old root, to make them available for GC
	oldRoot.Children = nil
	return true
}

// Remove previous tree & update game ops state
func (mcts *MCTS[T, S, R, O, A]) Reset(isTerminated bool, defaultStats S) {
	// Discard running search
	if mcts.IsSearching() {
		mcts.Stop()
		mcts.Synchronize()
	}

	// Reset game state and make new root
	mcts.ops.Reset()
	mcts.Root = newRootNode[T](isTerminated, defaultStats)
	mcts.size.Store(1)
	mcts.Root.CanExpand()
	mcts.Root.FinishExpanding()

	// insignificant optimization
	if !isTerminated {
		mcts.size.Add(mcts.ops.ExpandNode(mcts.Root))
	}
}

// 'the best move' in the position
func (mcts *MCTS[T, S, R, O, A]) BestMove() T {
	var signature T
	if bestChild := mcts.BestChild(mcts.Root, BestChildMostVisits); bestChild != nil {
		signature = bestChild.Move
	}
	return signature
}

// Current evaluation of the position
func (mcts *MCTS[T, S, R, O, A]) RootScore() Result {
	if bestChild := mcts.BestChild(mcts.Root, BestChildMostVisits); bestChild != nil {
		return bestChild.Stats.Q() / Result(bestChild.Stats.N())
	}
	return Result(math.NaN())
}

// Return best child, based on the policy
func (mcts *MCTS[T, S, R, O, A]) BestChild(node *NodeBase[T, S], policy BestChildPolicy) *NodeBase[T, S] {
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
	// fmt.Printf(" wr=%0.2f\n", float64(node.Stats.Q())/float64(node.Stats.N()))
	// for i := range node.Children {
	// 	ch := &node.Children[i]
	// 	fmt.Printf("%d. %v v=%d (wr=%.2f)\n",
	// 		i+1, ch.Move, ch.Stats.N(),
	// 		float64(ch.Stats.Q())/float64(ch.Stats.N()),
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
			maxVisits = max(int(node.Children[i].Stats.N()), maxVisits)
		}

		// Go through the children
		for i := 0; i < len(node.Children); i++ {
			child = &node.Children[i]
			real := child.Stats.RealVisits()
			if real > minVisitsThreshold && real > int32(minVisitsPercentageThreshold*float64(maxVisits)) {

				// We optimize the winning chances, looking from the root's perspective
				var winRate float64 = float64(child.Stats.Q()) / float64(child.Stats.N())

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

type PvResult[T MoveLike, S NodeStatsLike[S]] struct {
	Root     *NodeBase[T, S]
	Pv       []T
	Terminal bool
	Draw     bool
}

// Returns 'pvCount' best move lines, specified in the limits
func (mcts *MCTS[T, S, R, O, A]) MultiPv(policy BestChildPolicy) []PvResult[T, S] {
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
		va, vb := a.Stats.N(), b.Stats.N()
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
func (mcts *MCTS[T, S, R, O, A]) PvNodes(root *NodeBase[T, S], policy BestChildPolicy, includeRoot bool) ([]*NodeBase[T, S], bool) {
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
func (mcts *MCTS[T, S, R, O, A]) Pv(root *NodeBase[T, S], policy BestChildPolicy, includeRoot bool) ([]T, bool, bool) {
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

	return pv, mate, (mate && node.Stats.AvgQ() == 0.5)
}
