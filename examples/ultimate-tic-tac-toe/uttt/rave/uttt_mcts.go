package rave_uttt

/*

Ultimate Tic Tac Toe MCTS implementation with UCB1 selection

*/

import (
	uttt "go-mcts/examples/ultimate-tic-tac-toe/uttt/core"
	"go-mcts/pkg/mcts"
	"math/rand"
	"time"
	"unsafe"
)

type UtttGameResult struct {
	omoves []uttt.PosType
	xmoves []uttt.PosType
	turn   uttt.TurnType
	result mcts.Result
}

func (r *UtttGameResult) Value() mcts.Result {
	return r.result
}

func (r *UtttGameResult) Moves() []uttt.PosType {
	mvs := r.xmoves
	if r.turn == uttt.CircleTurn {
		mvs = r.omoves
	}
	return mvs
}

func (r *UtttGameResult) Append(m uttt.PosType) {
	mvptr := &r.xmoves
	if r.turn == uttt.CircleTurn {
		mvptr = &r.omoves
	}
	*mvptr = append(*mvptr, m)
}

func (r *UtttGameResult) SwitchTurn() {
	r.turn = uttt.TurnType(!r.turn)
}

// Actual UTTT mcts implementation
type UtttMCTS struct {
	mcts.MCTS[uttt.PosType, *mcts.RaveStats, *UtttGameResult]
	ops *UtttOperations
}

type UtttNode mcts.NodeBase[uttt.PosType, *mcts.RaveStats]

func NewUtttMCTS(position uttt.Position) *UtttMCTS {
	// Each mcts instance must have its own operations instance
	uttt_ops := newUtttOps(position)
	tree := &UtttMCTS{
		MCTS: *mcts.NewMTCS(
			mcts.RAVE,
			mcts.RaveGameOperations[uttt.PosType, *mcts.RaveStats, *UtttGameResult](uttt_ops),
			mcts.TerminalFlag(position.IsTerminated()),
			mcts.MultithreadTreeParallel,
			&mcts.RaveStats{},
			mcts.RaveBackprop[uttt.PosType, *mcts.RaveStats, *UtttGameResult]{},
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
func (tree *UtttMCTS) Selection() *mcts.NodeBase[uttt.PosType, *mcts.RaveStats] {
	return tree.MCTS.Selection(tree.Root, tree.ops, rand.New(rand.NewSource(time.Now().UnixNano())), 0)
}

// Default backprop used for debugging
func (tree *UtttMCTS) Backpropagate(node *mcts.NodeBase[uttt.PosType, *mcts.RaveStats], result *UtttGameResult) {
	tree.MCTS.Strategy().Backpropagate(tree.ops, node, result)
}

func (tree *UtttMCTS) Ops() mcts.RaveGameOperations[uttt.PosType, *mcts.RaveStats, *UtttGameResult] {
	return tree.ops
}

func (tree *UtttMCTS) Reset() {
	tree.MCTS.Reset(tree.ops, tree.ops.position.IsTerminated(), &mcts.RaveStats{})
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
		Memory: uint64(unsafe.Sizeof(mcts.NodeBase[uttt.PosType, *mcts.RaveStats]{})) * uint64(tree.Size()),
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
	// Will be set by search thread, with 'SetRand'
	random *rand.Rand
}

func newUtttOps(pos uttt.Position) *UtttOperations {
	return &UtttOperations{
		position: pos,
		rootSide: pos.Turn(),
	}
}

func (ops *UtttOperations) Reset() {
	ops.rootSide = ops.position.Turn()
}

func (ops *UtttOperations) ExpandNode(node *mcts.NodeBase[uttt.PosType, *mcts.RaveStats]) uint32 {

	moves := ops.position.GenerateMoves()
	node.Children = make([]mcts.NodeBase[uttt.PosType, *mcts.RaveStats], moves.Size)

	for i, m := range moves.Slice() {
		ops.position.MakeMove(m)
		isTerminal := ops.position.IsTerminated()
		ops.position.UndoMove()

		node.Children[i] = *mcts.NewBaseNode(node, m, isTerminal, &mcts.RaveStats{})
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
func (ops *UtttOperations) Rollout() *UtttGameResult {
	var moves *uttt.MoveList
	var move uttt.PosType
	var result mcts.Result = 0.5
	var moveCount int = 0
	leafTurn := ops.position.Turn()
	xmoves := uttt.NewMoveList()
	omoves := uttt.NewMoveList()

	for !ops.position.IsTerminated() {
		moveCount++
		moves = ops.position.GenerateMoves()

		// Choose at random move
		move = moves.Moves[ops.random.Int31()%int32(moves.Size)]

		if ops.position.Turn() == uttt.CircleTurn {
			omoves.AppendMove(move)
		} else {
			xmoves.AppendMove(move)
		}

		ops.position.MakeMove(move)
		// movesPlayed.AppendMove(move)
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

	return &UtttGameResult{
		omoves: omoves.Slice(),
		xmoves: xmoves.Slice(),
		result: result,
		turn:   leafTurn,
	}
}

func (ops *UtttOperations) SetRand(r *rand.Rand) {
	ops.random = r
}

func (ops UtttOperations) Clone() mcts.GameOperations[uttt.PosType, *mcts.RaveStats, *UtttGameResult] {
	return mcts.GameOperations[uttt.PosType, *mcts.RaveStats, *UtttGameResult](&UtttOperations{
		position: ops.position.Clone(),
		rootSide: ops.rootSide,
	})
}
