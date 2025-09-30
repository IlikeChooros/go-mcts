package chessmcts

/*

Chess MCTS implementation using the 3rd party dylhunn's chess move generation package
(https://github.com/dylhunn/dragontoothmg)

*/

import (
	"math/rand"

	chess "github.com/IlikeChooros/dragontoothmg"
	mcts "github.com/IlikeChooros/go-mcts/pkg/mcts"
)

type UcbGameOps struct {
	board  *chess.Board
	random *rand.Rand
}

type UcbMctsType struct {
	mcts.MCTS[chess.Move, *mcts.NodeStats, mcts.Result]
	ops *UcbGameOps
}

func NewUcbMcts() *UcbMctsType {
	ops := newUcbGameOps()
	Mcts := &UcbMctsType{
		MCTS: *mcts.NewMTCS(
			mcts.UCB1,
			ops,
			0,
			mcts.MultithreadTreeParallel,
			&mcts.NodeStats{},
			mcts.DefaultBackprop[chess.Move, *mcts.NodeStats, mcts.Result]{},
		),
		ops: ops,
	}

	return Mcts
}

func (ucb *UcbMctsType) Search() {
	ucb.SearchMultiThreaded(ucb.ops)

	ucb.Synchronize()
}

func newUcbGameOps() *UcbGameOps {
	return &UcbGameOps{
		board: chess.NewBoard(),
	}
}

func (o *UcbGameOps) ExpandNode(p *mcts.NodeBase[chess.Move, *mcts.NodeStats]) uint32 {
	moves := o.board.GenerateLegalMoves()
	p.Children = make([]mcts.NodeBase[chess.Move, *mcts.NodeStats], len(moves))

	for i := range moves {
		o.board.Make(moves[i])
		isTerminal := o.board.IsTerminated(len(o.board.GenerateLegalMoves()))
		o.board.Undo()

		p.Children[i] = *mcts.NewBaseNode(p, moves[i], isTerminal, &mcts.NodeStats{})
	}

	return 0
}

func (o *UcbGameOps) Traverse(m chess.Move) {
	o.board.Make(m)
}

func (o *UcbGameOps) BackTraverse() {
	o.board.Undo()
}

func (o *UcbGameOps) Rollout() mcts.Result {
	var result mcts.Result = 0.5
	var moveCount int = 0
	leafIsWhite := o.board.Wtomove

	moves := o.board.GenerateLegalMoves()
	for !o.board.IsTerminated(len(moves)) {
		moveCount++

		o.board.Make(moves[o.random.Int()%len(moves)])
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

	return result
}

func (o *UcbGameOps) Reset() {}

func (o *UcbGameOps) SetRand(r *rand.Rand) {
	o.random = r
}

func (o *UcbGameOps) Clone() mcts.GameOperations[chess.Move, *mcts.NodeStats, mcts.Result] {
	return &UcbGameOps{

		board: o.board.Clone(),
	}
}
