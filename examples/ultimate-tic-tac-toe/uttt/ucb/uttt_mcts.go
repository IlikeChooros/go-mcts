package ucb_uttt

/*

Ultimate Tic Tac Toe MCTS implementation with UCB1 selection

*/

import (
	"math/rand"
	"time"
	"unsafe"

	uttt "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/core"
	mcts "github.com/IlikeChooros/go-mcts/pkg/mcts"
)

// Actual UTTT mcts implementation
type UtttMCTS struct {
	mcts.MCTS[uttt.PosType, *mcts.NodeStats, mcts.Result, *UtttOperations]
}

func NewUtttMCTS(position uttt.Position) *UtttMCTS {
	// Each mcts instance must have its own operations instance
	return &UtttMCTS{
		MCTS: *mcts.NewMTCS(
			mcts.UCB1,
			NewUtttOps(position),
			mcts.MultithreadTreeParallel,
			&mcts.NodeStats{},
			mcts.DefaultBackprop[uttt.PosType, *mcts.NodeStats, mcts.Result, *UtttOperations]{},
		),
	}
}

// Start the search
func (tree *UtttMCTS) Search() {

	// Run the search
	tree.SearchMultiThreaded()

	// Wait for the search to end
	tree.Synchronize()
}

func (tree *UtttMCTS) Reset() {
	tree.MCTS.Reset(tree.Ops().position.IsTerminated(), &mcts.NodeStats{})
}

func (tree *UtttMCTS) SetPosition(position uttt.Position) {
	tree.Ops().position = position
	tree.Reset()
}

func (mcts *UtttMCTS) SetNotation(notation string) error {
	defer mcts.Reset()
	return mcts.Ops().position.FromNotation(notation)
}

func (tree *UtttMCTS) SearchResult(pvPolicy mcts.BestChildPolicy) uttt.SearchResult {

	multipv := tree.MultiPv(pvPolicy)
	result := uttt.SearchResult{
		Cps:    tree.Cps(),
		Depth:  tree.MaxDepth(),
		Cycles: tree.Root.Stats.N(),
		Lines:  make([]uttt.EngineLine, len(multipv)),
		Turn:   tree.Ops().rootSide,
		Size:   tree.Size(),
		Memory: uint64(unsafe.Sizeof(mcts.NodeBase[uttt.PosType, *mcts.NodeStats]{})) * uint64(tree.Size()),
	}

	for i := range len(multipv) {
		pvResult := multipv[i]
		line := &result.Lines[i]
		line.Pv = pvResult.Pv

		// Set the score
		if pvResult.Terminal {
			if pvResult.Draw {
				line.ScoreType = uttt.ValueScore
				line.Value = 50
			} else {
				line.ScoreType = uttt.MateScore
				line.Value = len(pvResult.Pv)

				// If the game ends on our turn, we are losing
				if line.Value%2 == 0 {
					line.Value = -line.Value
				}
			}
		} else {
			line.ScoreType = uttt.ValueScore
			if pvResult.Root.Stats.N() == 0 {
				line.Value = 50
			} else {
				line.Value = int(100 * pvResult.Root.Stats.AvgQ())
			}
		}
	}
	return result
}

// Must meet mcts.GameOperations
// That is:
//
// - Reset() - called in tree's Reset method when discarding current search,
// useful for internal state reset (like the rootSide = postion.Turn())
//
// - ExpandNode() - which must acutally append children to provided node,
// use the mcts.NewBaseNode() function for proper node initialization
//
// - Traverse(move) - called to update the internal position state, when traversing
// the tree
//
// - BackTraverse() - simply undo the last move, position object should hold a history of
// moves, since that function will be called repeatedly
//
// - Rollout() Result - Plays a game until a terminal node is reached, assigns a result based
// on the starting node's perspective
//
// optional:
//
// - SetRand(rand.Rand) - (from math.rand), sets a random generator created in the search thread.
// Add this function if you want to perform light playouts (making random moves)
type UtttOperations struct {
	position uttt.Position
	// This is needed for the SearchResult to work properly, since
	// I allow calling that function during the search (ops.position.Turn() may return wrong one)
	rootSide uttt.TurnType
	// Will be set by search thread, with 'SetRand'
	random *rand.Rand
}

func NewUtttOps(pos uttt.Position) *UtttOperations {
	return &UtttOperations{
		position: pos,
		rootSide: pos.Turn(),
		random:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (ops *UtttOperations) Reset() {
	ops.rootSide = ops.position.Turn()
}

func (ops *UtttOperations) ExpandNode(node *mcts.NodeBase[uttt.PosType, *mcts.NodeStats]) uint32 {

	moves := ops.position.GenerateMoves()
	node.Children = make([]mcts.NodeBase[uttt.PosType, *mcts.NodeStats], moves.Size)

	for i, m := range moves.Slice() {
		ops.position.MakeMove(m)
		isTerminal := ops.position.IsTerminated()
		ops.position.Undo()

		node.Children[i] = *mcts.NewBaseNode(node, m, isTerminal, &mcts.NodeStats{})
	}

	return uint32(moves.Size)
}

func (ops *UtttOperations) Traverse(move uttt.PosType) {
	ops.position.MakeMove(move)
}

func (ops *UtttOperations) BackTraverse() {
	ops.position.Undo()
}

// Play the game until a terminal node is reached
// The result is relative to the 'starting' node of the rollout
func (ops *UtttOperations) Rollout() mcts.Result {
	var moves *uttt.MoveList
	var move uttt.PosType
	var result mcts.Result = 0.5
	var moveCount int = 0
	leafTurn := ops.position.Turn()

	for !ops.position.IsTerminated() {
		moveCount++
		moves = ops.position.GenerateMoves()

		// Choose at random move
		move = moves.Moves[ops.random.Int31()%int32(moves.Size)]
		ops.position.MakeMove(move)
	}

	// If that's not a draw
	if t := ops.position.Termination(); (t == uttt.TerminationCircleWon && leafTurn == uttt.CircleTurn) ||
		(t == uttt.TerminationCrossWon && leafTurn == uttt.CrossTurn) {
		result = 1.0
		// We lost
	} else if t != uttt.TerminationDraw {
		result = 0.0
	}

	// Undo the moves
	for range moveCount {
		ops.position.Undo()
	}

	return result
}

func (ops *UtttOperations) SetRand(r *rand.Rand) {
	ops.random = r
}

func (ops UtttOperations) Clone() *UtttOperations {
	return &UtttOperations{
		position: *ops.position.Clone(),
		rootSide: ops.rootSide,
	}
}

// Added for benchmarking purposes
func (ops *UtttOperations) Position() *uttt.Position {
	return &ops.position
}

func (ops *UtttOperations) SetPosition(pos uttt.Position) {
	ops.position = pos
	ops.rootSide = pos.Turn()
}
