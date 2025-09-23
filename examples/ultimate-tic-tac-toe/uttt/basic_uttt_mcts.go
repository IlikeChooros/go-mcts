package basic_uttt_mcts

/*

Ultimate Tic Tac Toe MCTS implementation

*/

import (
	uttt "go-mcts/examples/ultimate-tic-tac-toe/uttt/core"
	"go-mcts/pkg/mcts"
	"math/rand"
	"time"
	"unsafe"
)

// Actual UTTT mcts implementation
type UtttMCTS struct {
	mcts.MCTS[uttt.PosType, *mcts.NodeStats]
	ops *UtttOperations
}

type UtttNode mcts.NodeBase[uttt.PosType, *mcts.NodeStats]

func NewUtttMCTS(position uttt.Position) *UtttMCTS {
	// Each mcts instance must have its own operations instance
	uttt_ops := newUtttOps(position)
	tree := &UtttMCTS{
		MCTS: *mcts.NewMTCS(
			mcts.UCB1,
			mcts.GameOperations[uttt.PosType, *mcts.NodeStats](uttt_ops),
			mcts.TerminalFlag(position.IsTerminated()),
			mcts.MultithreadTreeParallel,
			&mcts.NodeStats{},
		),
		ops: uttt_ops,
	}
	return tree
}

func (tree *UtttMCTS) AsyncSearch() {
	tree.MCTS.SearchMultiThreaded(tree.ops)
}

// Start the search
func (tree *UtttMCTS) Search() {

	// Run the search
	tree.SearchMultiThreaded(tree.ops)

	// Wait for the search to end
	tree.Synchronize()
}

// Default selection used for debugging
func (tree *UtttMCTS) Selection() *mcts.NodeBase[uttt.PosType, *mcts.NodeStats] {
	return tree.MCTS.Selection(tree.Root, tree.ops, rand.New(rand.NewSource(time.Now().UnixNano())), 0)
}

// Default backprop used for debugging
func (tree *UtttMCTS) Backpropagate(node *mcts.NodeBase[uttt.PosType, *mcts.NodeStats], result mcts.Result) {
	tree.MCTS.Backpropagate(tree.ops, node, result)
}

func (tree *UtttMCTS) Ops() mcts.GameOperations[uttt.PosType, *mcts.NodeStats] {
	return tree.ops
}

func (tree *UtttMCTS) Reset() {
	tree.MCTS.Reset(tree.ops, tree.ops.position.IsTerminated(), &mcts.NodeStats{})
}

// Set the position
func (tree *UtttMCTS) SetPosition(position uttt.Position) {
	tree.ops.position = position
	tree.Reset()
}

func (mcts *UtttMCTS) SetNotation(notation string) error {
	defer mcts.Reset()
	return mcts.ops.position.FromNotation(notation)
}

func (tree *UtttMCTS) SearchResult(pvPolicy mcts.BestChildPolicy) uttt.SearchResult {

	multipv := tree.MultiPv(pvPolicy)
	result := uttt.SearchResult{
		Cps:    tree.Cps(),
		Depth:  tree.MaxDepth(),
		Cycles: tree.Root.Stats.Visits(),
		Lines:  make([]uttt.EngineLine, len(multipv)),
		Turn:   tree.ops.rootSide,
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
			if pvResult.Root.Stats.Visits() == 0 {
				line.Value = 50
			} else {
				line.Value = int(100 * pvResult.Root.Stats.AvgOutcome())
			}
		}
	}
	return result
}

type UtttOperations struct {
	position uttt.Position
	rootSide uttt.TurnType
	random   *rand.Rand
}

func newUtttOps(pos uttt.Position) *UtttOperations {
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
		ops.position.UndoMove()

		node.Children[i] = *mcts.NewBaseNode(node, m, isTerminal, &mcts.NodeStats{})
	}

	return uint32(moves.Size)
}

func (ops *UtttOperations) Traverse(signature uttt.PosType) {
	ops.position.MakeMove(signature)
}

func (ops *UtttOperations) BackTraverse() {
	ops.position.UndoMove()
}

// Play the game until a terminal node is reached
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
		ops.position.UndoMove()
	}

	return result
}

func (ops UtttOperations) Clone() mcts.GameOperations[uttt.PosType, *mcts.NodeStats] {
	return mcts.GameOperations[uttt.PosType, *mcts.NodeStats](&UtttOperations{
		position: ops.position.Clone(),
		rootSide: ops.rootSide,
		random:   rand.New(rand.NewSource(time.Now().UnixMicro())),
	})
}
