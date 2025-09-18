package mcts

import (
	"fmt"
	"math"
	"slices"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Generalized Monte-Carlo Tree Search algorithm

var VirtualLoss int32 = 0

// Result of the rollout, should range from [0, 1] - 0 being loss from the leaf's node perspective
// and 1 being a win
type Result float64
type MoveLike comparable
type BestChildPolicy int

var ExplorationParam float64 = 0.75

// Set the exploration parameter used in UCB1 formula
func SetExplorationParam(c float64) {
	ExplorationParam = max(0.0, c)
}

const (
	BestChildMostVisits BestChildPolicy = iota
	BestChildWinRate
)

type MultithreadPolicy int

const (
	// Parallel building of the same game tree, protecting data from simultaneous writes
	// using atomic operations.
	MultithreadTreeParallel MultithreadPolicy = iota

	// This will spawn multiple threads (specified by Limits.NThreads) that will
	// build independent game trees in parallel and making the move basing on
	// the root-level branches of all these trees. Note that listeners will be called
	// only on the main thread, so the evaluation and pv will be inaccurate until
	// the results are merged after the search is done.
	MultithreadRootParallel = iota
)

// Will be called, when we choose this node, as it is the most promising to expand
// Warning: when using NodeStats fields, must use atomic operations (Load, Store)
// since the search may be multi-threaded (tree parallelized)
type SelectionPolicy[T MoveLike] func(parent, root *NodeBase[T]) *NodeBase[T]

type GameOperations[T MoveLike] interface {
	// Generate moves here, and add them as children to given node
	ExpandNode(parent *NodeBase[T]) uint32
	// Make a move on the internal position definition, with given
	// signature value (move)
	Traverse(T)
	// Go back up 1 time in the game tree (undo previous move, which was played in traverse)
	BackTraverse()
	// Function to make the playout, until terminal node is reached,
	// in case of UTTT, play random moves, until we reach draw/win/loss
	Rollout() Result
	// Reset game state to current internal position, called after changing
	// position, for example using SetNotation function in engine
	Reset()
	// Clone itself, without any shared memory with the other object
	Clone() GameOperations[T]
}

type TreeStats struct {
	// size     atomic.Int32
	maxdepth atomic.Int32
	cps      atomic.Uint32
	nodes    atomic.Uint32
}

type MCTS[T MoveLike] struct {
	TreeStats
	listener          *StatsListener[T]
	Limiter           LimiterLike
	selection_policy  SelectionPolicy[T]
	Root              *NodeBase[T]
	size              atomic.Uint32
	wg                sync.WaitGroup
	collisionCount    atomic.Int32
	multithreadPolicy MultithreadPolicy
	roots             []*NodeBase[T]
	merged            atomic.Bool
}

// Create new base tree
func NewMTCS[T MoveLike](
	selectionPolicy SelectionPolicy[T],
	operations GameOperations[T],
	flags uint32,
	multithreadPolicy MultithreadPolicy,
) *MCTS[T] {
	mcts := &MCTS[T]{
		TreeStats:         TreeStats{},
		listener:          &StatsListener[T]{},
		Limiter:           LimiterLike(NewLimiter(uint32(unsafe.Sizeof(NodeBase[T]{})))),
		selection_policy:  selectionPolicy,
		Root:              &NodeBase[T]{Flags: flags},
		multithreadPolicy: multithreadPolicy,
	}

	// Set IsThinking to false
	mcts.Limiter.Stop()

	// Expand the root node, by default
	if mcts.Root.CanExpand() {
		mcts.Root.FinishExpanding()
		mcts.size.Store(1 + operations.ExpandNode(mcts.Root))
	} else {
		mcts.size.Store(1)
	}

	return mcts
}

func (mcts *MCTS[T]) invokeListener(f ListenerFunc[T]) {
	if f != nil {
		f(toListenerStats(mcts))
	}
}

// Get the collision count, which is the number of times
// the node was chosen, but it was already being expanded
// resulting in a 'waiting' state
func (mcts *MCTS[T]) CollisionCount() int32 {
	return mcts.collisionCount.Load()
}

// Number of all collisions in the tree divided by the number of all cycles
func (mcts *MCTS[T]) CollisionFactor() float64 {
	if mcts.Nodes() == 0 {
		return 0.0
	}
	return float64(mcts.collisionCount.Load()) / float64(mcts.Root.Visits())
}

func (mcts *MCTS[T]) ResetListener() {
	mcts.listener.OnCycle(nil).OnDepth(nil).OnStop(nil)
}

func (mcts *MCTS[T]) StatsListener() *StatsListener[T] {
	return mcts.listener
}

func (mcts *MCTS[T]) IsThinking() bool {
	return !mcts.Limiter.Stop()
}

func (mcts *MCTS[T]) Stop() {
	mcts.Limiter.SetStop(true)
}

func (mcts *MCTS[T]) MaxDepth() int {
	return int(mcts.maxdepth.Load())
}

func (mcts *MCTS[T]) Cycles() int {
	if mcts.multithreadPolicy == MultithreadTreeParallel || mcts.merged.Load() {
		return int(mcts.Root.Visits())
	}
	// In root parallel and not merged, sum all the root visits
	cycles := 0
	for i := range mcts.roots {
		cycles += int(mcts.roots[i].Visits())
	}
	return cycles
}

func (mcts *MCTS[T]) Cps() uint32 {
	return mcts.cps.Load()
}

func (mcts *MCTS[T]) Nodes() uint32 {
	return mcts.nodes.Load()
}

// Get the reason why the search was stopped, valid after search ends
func (mcts *MCTS[T]) StopReason() StopReason {
	return mcts.Limiter.StopReason()
}

func (mcts *MCTS[T]) SetLimits(limits *Limits) {
	mcts.Limiter.SetLimits(limits)
}

func (mcts *MCTS[T]) Limits() *Limits {
	return mcts.Limiter.Limits()
}

func (mcts *MCTS[T]) String() string {
	str := fmt.Sprintf("MCTS={Size=%d, Stats:{MaxDepth=%d, cps=%d, Nodes=%d}, Stop=%v",
		mcts.Size(), mcts.MaxDepth(), mcts.Cps(), mcts.Nodes(), !mcts.IsThinking())
	str += fmt.Sprintf(", Root=%v, Root.Children=%v", mcts.Root, mcts.Root.Children)
	return str
}

// Helper function to count tree nodes
func countTreeNodes[T MoveLike](node *NodeBase[T]) int {
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
func (mcts *MCTS[T]) Count() int {
	return countTreeNodes(mcts.Root)
}

// Get the size of the tree
func (mcts *MCTS[T]) Size() uint32 {
	// Count every node in the tree
	return mcts.size.Load()
}

// Remove previous tree & update game ops state
func (mcts *MCTS[T]) Reset(ops GameOperations[T], isTerminated bool) {
	// Discard running search
	if mcts.IsThinking() {
		mcts.Stop()
		mcts.Synchronize()
	}

	// Reset game state and make new root
	ops.Reset()
	mcts.Root = newRootNode[T](isTerminated)
	mcts.size.Store(1)
	mcts.Root.CanExpand()
	mcts.Root.FinishExpanding()

	if !isTerminated {
		mcts.size.Add(ops.ExpandNode(mcts.Root))
	}
}

// 'the best move' in the position
func (mcts *MCTS[T]) RootSignature() T {
	var signature T
	if bestChild := mcts.BestChild(mcts.Root, BestChildMostVisits); bestChild != nil {
		signature = bestChild.NodeSignature
	}
	return signature
}

// Current evaluation of the position
func (mcts *MCTS[T]) RootScore() Result {
	if bestChild := mcts.BestChild(mcts.Root, BestChildMostVisits); bestChild != nil {
		return bestChild.Outcomes() / Result(bestChild.Visits())
	}
	return Result(math.NaN())
}

// Return best child, based on the number of visits
func (mcts *MCTS[T]) BestChild(node *NodeBase[T], policy BestChildPolicy) *NodeBase[T] {
	var bestChild *NodeBase[T]
	var child *NodeBase[T]
	maxVisits := 0

	// DEBUG
	// rootTurn := mcts.Root.Turn() == node.Turn()
	// if rootTurn {
	// 	fmt.Print("Root's turn")
	// } else {
	// 	fmt.Print("Enemy's turn")
	// }
	// fmt.Printf(" wr=%0.2f\n", float64(node.Outcomes())/float64(node.Visits()))
	// for i := range node.Children {
	// 	ch := &node.Children[i]
	// 	fmt.Printf("%d. %v v=%d (wr=%.2f)\n",
	// 		i+1, ch.NodeSignature, ch.Visits(),
	// 		float64(ch.Outcomes())/float64(ch.Visits()),
	// 	)
	// }

	switch policy {
	case BestChildMostVisits:
		for i := 0; i < len(node.Children); i++ {
			child = &node.Children[i]
			if v := int(child.RealVisits()); v > maxVisits && v > 0 {
				maxVisits = int(child.RealVisits())
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
			maxVisits = max(int(node.Children[i].Visits()), maxVisits)
		}

		// Go through the children
		for i := 0; i < len(node.Children); i++ {
			child = &node.Children[i]
			real := child.RealVisits()
			if real > minVisitsThreshold && real > int32(minVisitsPercentageThreshold*float64(maxVisits)) {

				// We optimize the winning chances, looking from the root's perspective
				var winRate float64 = float64(child.Outcomes()) / float64(child.Visits())

				if winRate > bestWinRate {
					bestWinRate = winRate
					bestChild = child
				}
			}
		}
	}

	// if bestChild != nil {
	// 	fmt.Println("Chose", bestChild.NodeSignature)
	// }

	return bestChild
}

type PvResult[T MoveLike] struct {
	Root     *NodeBase[T]
	Pv       []T
	Terminal bool
	Draw     bool
}

// Returns 'pvCount' best move lines
func (mcts *MCTS[T]) MultiPv(policy BestChildPolicy) []PvResult[T] {
	if mcts.Root == nil {
		return nil
	}

	pvCount := mcts.Limiter.Limits().MultiPv
	multipv := make([]PvResult[T], 0, pvCount)
	child_count := len(mcts.Root.Children)
	root_nodes := make([]*NodeBase[T], child_count)
	for i := range child_count {
		root_nodes[i] = &mcts.Root.Children[i]
	}

	slices.SortFunc(root_nodes, func(a *NodeBase[T], b *NodeBase[T]) int {
		va, vb := a.Visits(), b.Visits()
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
			multipv = append(multipv, PvResult[T]{
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
func (mcts *MCTS[T]) PvNodes(root *NodeBase[T], policy BestChildPolicy, includeRoot bool) ([]*NodeBase[T], bool) {
	if root == nil {
		return nil, false
	}

	pv := make([]*NodeBase[T], 0, mcts.MaxDepth()+1)
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

// Get the pricipal variation, but only the moves
func (mcts *MCTS[T]) Pv(root *NodeBase[T], policy BestChildPolicy, includeRoot bool) ([]T, bool, bool) {
	if root == nil {
		return nil, false, false
	}

	var node *NodeBase[T]
	nodes, mate := mcts.PvNodes(root, policy, includeRoot)
	pv := make([]T, len(nodes))
	for i := range len(nodes) {
		node = nodes[i]
		pv[i] = node.NodeSignature
	}

	return pv, mate, (mate && node.AvgOutcome() == 0.5)
}
