package chessmcts

/*

Chess MCTS implementation using the 3rd party dylhunn's chess move generation package
(https://github.com/dylhunn/dragontoothmg)

Using RAVE as selection policy

*/

import (
	"math/rand"

	chess "github.com/IlikeChooros/dragontoothmg"
	mcts "github.com/IlikeChooros/go-mcts/pkg/mcts"
)

type RaveGameOps struct {
	board  *chess.Board
	random *rand.Rand
}

// Must meet mcts.RaveGameResult
type RaveGameResult struct {
	wmoves  []chess.Move
	bmoves  []chess.Move
	wtomove bool
	result  mcts.Result
}

// The result of the playout [0, 1]: 1 - this side won, 0 - this side lost
func (r *RaveGameResult) Value() mcts.Result {
	return r.result
}

// Returns moves played in rollout (playout), so that they match
// with current player's turn
func (r *RaveGameResult) Moves() []chess.Move {
	mvs := r.bmoves
	if r.wtomove {
		mvs = r.wmoves
	}
	return mvs
}

// During backpropagation, moves will be appended
// to match with the tree's selected path
func (r *RaveGameResult) Append(m chess.Move) {
	mvptr := &r.bmoves
	if r.wtomove {
		mvptr = &r.wmoves
	}
	*mvptr = append(*mvptr, m)
}

// This will be called in backpropagation of the game result.
// It is mainly for switching the moves that were played in the rollout,
// so that they match with current's player turn.
func (r *RaveGameResult) SwitchTurn() {
	r.wtomove = !r.wtomove
}

type RaveMctsType struct {
	mcts.MCTS[chess.Move, *mcts.RaveStats, *RaveGameResult]
	ops *RaveGameOps
}

func NewRaveMcts() *RaveMctsType {
	ops := newRaveGameOps()
	Mcts := &RaveMctsType{
		MCTS: *mcts.NewMTCS(
			mcts.RAVE,
			ops,
			0,
			mcts.MultithreadTreeParallel,
			&mcts.RaveStats{},
			mcts.RaveBackprop[chess.Move, *mcts.RaveStats, *RaveGameResult]{},
		),
		ops: ops,
	}

	return Mcts
}

func (ucb *RaveMctsType) Search() {
	ucb.SearchMultiThreaded(ucb.ops)

	ucb.Synchronize()
}

func newRaveGameOps() *RaveGameOps {
	return &RaveGameOps{
		board: chess.NewBoard(),
	}
}

func (o *RaveGameOps) ExpandNode(p *mcts.NodeBase[chess.Move, *mcts.RaveStats]) uint32 {
	moves := o.board.GenerateLegalMoves()
	p.Children = make([]mcts.NodeBase[chess.Move, *mcts.RaveStats], len(moves))

	for i := range moves {
		o.board.Make(moves[i])
		isTerminal := o.board.IsTerminated(len(o.board.GenerateLegalMoves()))
		o.board.Undo()
		p.Children[i] = *mcts.NewBaseNode(p, moves[i], isTerminal, &mcts.RaveStats{})
	}

	return uint32(len(moves))
}

func (o *RaveGameOps) Traverse(m chess.Move) {
	o.board.Make(m)
}

func (o *RaveGameOps) BackTraverse() {
	o.board.Undo()
}

func (o *RaveGameOps) Rollout() *RaveGameResult {
	var result mcts.Result = 0.5
	var moveCount int = 0
	leafIsWhite := o.board.Wtomove
	wmoves := make([]chess.Move, 0, 128)
	bmoves := make([]chess.Move, 0, 128)

	moves := o.board.GenerateLegalMoves()
	for !o.board.IsTerminated(len(moves)) {
		moveCount++

		m := moves[o.random.Int()%len(moves)]
		if o.board.Wtomove {
			wmoves = append(wmoves, m)
		} else {
			bmoves = append(bmoves, m)
		}

		o.board.Make(m)
		moves = o.board.GenerateLegalMoves()
	}

	// Game ended in checkmate
	if o.board.Termination() == chess.TerminationCheckmate {
		// Ended on our turn, so we lost
		if o.board.Wtomove == leafIsWhite {
			result = 0.0
		} else {
			result = 1.0
		}
	}
	// Else that's a draw, so no need to change the result

	for i := moveCount - 1; i >= 0; i-- {
		o.board.Undo()
	}

	return &RaveGameResult{
		result: result, wtomove: leafIsWhite,
		wmoves: wmoves, bmoves: bmoves,
	}
}

func (o *RaveGameOps) Reset() {}

func (o *RaveGameOps) SetRand(r *rand.Rand) {
	o.random = r
}

func (o *RaveGameOps) Clone() mcts.GameOperations[chess.Move, *mcts.RaveStats, *RaveGameResult] {
	return &RaveGameOps{
		board: o.board.Clone(),
	}
}
