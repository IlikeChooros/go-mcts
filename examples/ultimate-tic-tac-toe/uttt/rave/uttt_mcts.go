package rave_uttt

/*

Ultimate Tic Tac Toe MCTS implementation with RAVE selection

The code somewhat differs from the UCB example, mainly we have to manually
implement GameResult (UtttGameResult) and return it in Rollout (GameOperations).

Also in the Rollout, besides returning the float value of the result, moves that were
played also must be returned.

*/

import (
	"math/rand"
	"unsafe"

	uttt "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/core"
	mcts "github.com/IlikeChooros/go-mcts/pkg/mcts"
)

// Must meet mcts.RaveGameResult
type UtttGameResult struct {
	omoves []uttt.PosType
	xmoves []uttt.PosType
	turn   uttt.TurnType
	result mcts.Result
}

// The result of the playout [0, 1]: 1 - this side won, 0 - this side lost
func (r *UtttGameResult) Value() mcts.Result {
	return r.result
}

// Returns moves played in rollout (playout), so that they match
// with current player's turn
func (r *UtttGameResult) Moves() []uttt.PosType {
	mvs := r.xmoves
	if r.turn == uttt.CircleTurn {
		mvs = r.omoves
	}
	return mvs
}

// During backpropagation, moves will be appended
// to match with the tree's selected path
func (r *UtttGameResult) Append(m uttt.PosType) {
	mvptr := &r.xmoves
	if r.turn == uttt.CircleTurn {
		mvptr = &r.omoves
	}
	*mvptr = append(*mvptr, m)
}

// This will be called in backpropagation of the game result.
// It is mainly for switching the moves that were played in the rollout,
// so that they match with current's player turn.
func (r *UtttGameResult) SwitchTurn() {
	r.turn = uttt.TurnType(!r.turn)
}

// Actual UTTT mcts implementation
type UtttMCTS struct {
	// Using mcts.RaveStats (which meet the mcts.RaveStatsLike interface)
	// and UtttGameResult (meets the mcts.RaveGameResult[uttt.PosType])
	mcts.MCTS[uttt.PosType, *mcts.RaveStats, *UtttGameResult]
	ops *UtttOperations
}

func NewUtttMCTS(position uttt.Position) *UtttMCTS {
	// Each mcts instance must have its own operations instance
	uttt_ops := newUtttOps(position)
	tree := &UtttMCTS{
		MCTS: *mcts.NewMTCS(
			mcts.RAVE,
			uttt_ops,
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

// Remove current game tree, resets the tree's and game ops's state
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

		// Choose at random move, the ops.random is non-nil, because it was
		// set in the begining of the search (with the SetRand method)
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

// Sets the random number generator, called at the begining of the search
func (ops *UtttOperations) SetRand(r *rand.Rand) {
	ops.random = r
}

// It should return a deep copy of the ops object
func (ops UtttOperations) Clone() mcts.GameOperations[uttt.PosType, *mcts.RaveStats, *UtttGameResult] {
	return mcts.GameOperations[uttt.PosType, *mcts.RaveStats, *UtttGameResult](&UtttOperations{
		position: ops.position.Clone(),
	})
}
