package main

/*

Ultimate Tic-Tac-Toe benchmarking example

Uses UCB1 and RAVE implementations to play against each other in an arena setup

*/

import (
	"math"

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

type baseMCTS[S mcts.NodeStatsLike[S], R mcts.GameResult, G utttOpsLike[S, R, G], A mcts.StrategyLike[uttt.PosType, S, R, G]] struct {
	mcts.MCTS[uttt.PosType, S, R, G, A]
}

func (b *baseMCTS[S, R, G, A]) Search() uttt.PosType {
	b.SearchMultiThreaded()
	b.Synchronize()
	return b.BestMove()
}

type ucbMCTS struct {
	baseMCTS[*mcts.NodeStats, mcts.Result, *ucb.UtttOperations, *mcts.UCB1[uttt.PosType, *mcts.NodeStats, mcts.Result, *ucb.UtttOperations]]
}

type raveMCTS struct {
	baseMCTS[*mcts.RaveStats, *rave.UtttGameResult, *rave.UtttOperations, *mcts.RAVE[uttt.PosType, *mcts.RaveStats, *rave.UtttGameResult, *rave.UtttOperations]]
}

func NewUcb() *ucbMCTS {
	// Each mcts instance must have its own operations instance
	return &ucbMCTS{
		baseMCTS: baseMCTS[*mcts.NodeStats, mcts.Result, *ucb.UtttOperations, *mcts.UCB1[uttt.PosType, *mcts.NodeStats, mcts.Result, *ucb.UtttOperations]]{
			MCTS: *mcts.NewMTCS(
				mcts.NewUCB1[uttt.PosType, *mcts.NodeStats, mcts.Result, *ucb.UtttOperations](0.45),
				ucb.NewUtttOps(*uttt.NewPosition()),
				mcts.MultithreadTreeParallel,
				&mcts.NodeStats{},
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
		baseMCTS: baseMCTS[*mcts.RaveStats, *rave.UtttGameResult, *rave.UtttOperations, *mcts.RAVE[uttt.PosType, *mcts.RaveStats, *rave.UtttGameResult, *rave.UtttOperations]]{
			MCTS: *mcts.NewMTCS(
				mcts.NewRAVE[uttt.PosType, *mcts.RaveStats, *rave.UtttGameResult, *rave.UtttOperations](),
				rave.NewUtttOps(*uttt.NewPosition()),
				mcts.MultithreadTreeParallel,
				&mcts.RaveStats{},
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
		maxThreads = 4     // threads per mcts instance (total threads = 2 * maxThreads * arenaThreads)
		totalGames = 100   // total games to play
		maxCycles  = 50000 // cycles/iterations per mcts instance

		arenaThreads = 2 // threads for the arena manager
	)

	ucbmcts := NewUcb()
	ravemcts := NewRave()

	// Fine tune UCB1 exploration parameter
	ucbmcts.Strategy().SetExplorationParam(0.4)

	// Fine tune RAVE parameters
	ravemcts.Strategy().SetBetaFunction(func(n, nRave int32) float64 {
		const K = 40000
		return math.Sqrt(K / (3.0*float64(n) + K))
	})
	ravemcts.Strategy().SetExplorationParam(0.2)

	// Setup and run the arena
	limits := mcts.DefaultLimits().SetThreads(maxThreads).SetCycles(maxCycles)
	arena := bench.NewVersusArena(uttt.NewPosition(),
		bench.ExtMCTS[uttt.PosType, *mcts.RaveStats, *rave.UtttGameResult, *uttt.Position](ravemcts),
		bench.ExtMCTS[uttt.PosType, *mcts.NodeStats, mcts.Result, *uttt.Position](ucbmcts),
	)
	arena.Setup(limits, totalGames, arenaThreads)

	// P1: UCB1
	// P2: RAVE
	arena.Start(&bench.DefaultListener[uttt.PosType]{})
	arena.Wait()
}
