package main

/*

Ultimate Tic-Tac-Toe benchmarking example

Uses UCB1 and RAVE implementations to play against each other in an arena setup

*/

import (
	uttt "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/core"
	rave "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/rave"
	ucb "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/ucb"
	"github.com/IlikeChooros/go-mcts/pkg/bench"
	"github.com/IlikeChooros/go-mcts/pkg/mcts"
)

type Position struct {
	uttt.Position
}

type utttOpsLike[S mcts.NodeStatsLike[S], R mcts.GameResult, G mcts.GameOperations[uttt.PosType, S, R, G]] interface {
	mcts.GameOperations[uttt.PosType, S, R, G]
	SetPosition(uttt.Position)
	Position() *uttt.Position
}

type baseMCTS[S mcts.NodeStatsLike[S], R mcts.GameResult, G utttOpsLike[S, R, G]] struct {
	mcts.MCTS[uttt.PosType, S, R, G]
}

func (b *baseMCTS[S, R, G]) Search() uttt.PosType {
	b.SearchMultiThreaded()
	b.Synchronize()
	return b.BestMove()
}

type ucbMCTS struct {
	baseMCTS[*mcts.NodeStats, mcts.Result, *ucb.UtttOperations]
}

type raveMCTS struct {
	baseMCTS[*mcts.RaveStats, *rave.UtttGameResult, *rave.UtttOperations]
}

func NewUcb() *ucbMCTS {
	// Each mcts instance must have its own operations instance
	return &ucbMCTS{
		baseMCTS: baseMCTS[*mcts.NodeStats, mcts.Result, *ucb.UtttOperations]{
			MCTS: *mcts.NewMTCS(
				mcts.UCB1,
				ucb.NewUtttOps(*uttt.NewPosition()),
				mcts.MultithreadTreeParallel,
				&mcts.NodeStats{},
				mcts.DefaultBackprop[uttt.PosType, *mcts.NodeStats, mcts.Result, *ucb.UtttOperations]{},
			),
		},
	}
}

func (u *ucbMCTS) Reset() {
	u.baseMCTS.Reset(u.baseMCTS.Ops().Position().IsTerminated(), &mcts.NodeStats{})
}

func (u *ucbMCTS) SetPosition(position *uttt.Position) {
	u.Ops().SetPosition(*position)
	u.Reset()
}

func (u *ucbMCTS) Clone() bench.ExtMCTS[uttt.PosType, *mcts.NodeStats, mcts.Result, *uttt.Position] {
	newMCTS := NewUcb()
	newMCTS.Limiter.SetLimits(u.Limiter.Limits())
	return newMCTS
}

func NewRave() *raveMCTS {
	// Each mcts instance must have its own operations instance
	return &raveMCTS{
		baseMCTS: baseMCTS[*mcts.RaveStats, *rave.UtttGameResult, *rave.UtttOperations]{
			MCTS: *mcts.NewMTCS(
				mcts.RAVE,
				rave.NewUtttOps(*uttt.NewPosition()),
				mcts.MultithreadTreeParallel,
				&mcts.RaveStats{},
				mcts.RaveBackprop[uttt.PosType, *mcts.RaveStats, *rave.UtttGameResult, *rave.UtttOperations]{},
			),
		},
	}
}

func (r *raveMCTS) Reset() {
	r.baseMCTS.Reset(r.baseMCTS.Ops().Position().IsTerminated(), &mcts.RaveStats{})
}

func (r *raveMCTS) SetPosition(position *uttt.Position) {
	r.Ops().SetPosition(*position)
	r.Reset()
}

func (r *raveMCTS) Clone() bench.ExtMCTS[uttt.PosType, *mcts.RaveStats, *rave.UtttGameResult, *uttt.Position] {
	newMCTS := NewRave()
	newMCTS.Limiter.SetLimits(r.Limiter.Limits())
	return newMCTS
}

func main() {
	const (
		moveTime   = 1500 // ms
		maxThreads = 4
		totalGames = 10
		maxCycles  = 50000

		arenaThreads = 2 // threads for the arena manager
	)

	limits := mcts.DefaultLimits().SetMovetime(moveTime).SetThreads(maxThreads).SetCycles(maxCycles)

	ucbmcts := NewUcb()
	ravemcts := NewRave()

	arena := bench.NewVersusArena(uttt.NewPosition(),
		bench.ExtMCTS[uttt.PosType, *mcts.NodeStats, mcts.Result, *uttt.Position](ucbmcts),
		bench.ExtMCTS[uttt.PosType, *mcts.RaveStats, *rave.UtttGameResult, *uttt.Position](ravemcts))
	arena.Setup(limits, totalGames, arenaThreads)
	arena.Start(&bench.DefaultListener[uttt.PosType]{})
	arena.Wait()
}
