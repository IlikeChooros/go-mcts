package bench

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IlikeChooros/go-mcts/pkg/mcts"
)

/*
Arena benchmark subpackage, allows to play a series of games between two
different MCTS[T, S, R, O, A]configurations.
*/

type VersusMatchResult int

const (
	VersusPl1Win VersusMatchResult = 1
	VersusPl2Win VersusMatchResult = -1
	VersusDraw   VersusMatchResult = 0
)

type PositionLike[T mcts.MoveLike, P any] interface {
	MakeMove(T)
	Undo()
	IsTerminated() bool
	IsDraw() bool
	Clone() P
}

type VersusArenaStats struct {
	p1Wins uint32
	p2Wins uint32
	draws  uint32
}

func (vas *VersusArenaStats) Total() int {
	return int(vas.P1Wins() + vas.P2Wins() + vas.Draws())
}

func (vas *VersusArenaStats) P1Wins() int {
	return int(atomic.LoadUint32(&vas.p1Wins))
}

func (vas *VersusArenaStats) P2Wins() int {
	return int(atomic.LoadUint32(&vas.p2Wins))
}

func (vas *VersusArenaStats) Draws() int {
	return int(atomic.LoadUint32(&vas.draws))
}

type VersusWorkerInfo[T mcts.MoveLike] struct {
	WorkerID      int
	NGames        int
	FinishedGames int
	GameMoveNum   int
	Moves         []T
	P1Wins        int
	P2Wins        int
	Draws         int
}

type VersusSummaryInfo struct {
	TotalGames int
	P1Wins     int
	P2Wins     int
	Draws      int
	Workers    int
}

type ExtMCTS[T mcts.MoveLike, S mcts.NodeStatsLike[S], R mcts.GameResult, P PositionLike[T, P]] interface {
	Reset()          // reset the tree, keeping the ops and limits
	MakeMove(T) bool // make a move in the root position, updating the tree
	SetLimits(*mcts.Limits)
	Search() T
	SetPosition(P)
	Clone() ExtMCTS[T, S, R, P]
}

type VersusArena[T mcts.MoveLike, P PositionLike[T, P], S1 mcts.NodeStatsLike[S1], R1 mcts.GameResult, S2 mcts.NodeStatsLike[S2], R2 mcts.GameResult] struct {
	VersusArenaStats
	Player1  ExtMCTS[T, S1, R1, P]
	Player2  ExtMCTS[T, S2, R2, P]
	NGames   uint
	NThreads uint
	Limits   *mcts.Limits
	Position P
	wg       sync.WaitGroup
	finished atomic.Bool
	ctx      context.Context
}

func NewVersusArena[
	T mcts.MoveLike, P PositionLike[T, P], S1 mcts.NodeStatsLike[S1],
	R1 mcts.GameResult, S2 mcts.NodeStatsLike[S2], R2 mcts.GameResult](
	position P, tree1 ExtMCTS[T, S1, R1, P], tree2 ExtMCTS[T, S2, R2, P],
) *VersusArena[T, P, S1, R1, S2, R2] {
	return &VersusArena[T, P, S1, R1, S2, R2]{
		Player1:  tree1,
		Player2:  tree2,
		NGames:   100,
		NThreads: 2,
		Limits:   mcts.DefaultLimits().SetMovetime(1000),
		Position: position,
		ctx:      context.Background(),
	}
}

func (va *VersusArena[T, P, S1, R1, S2, R2]) WithContext(ctx context.Context) *VersusArena[T, P, S1, R1, S2, R2] {
	va.ctx = ctx
	return va
}

func (va *VersusArena[T, P, S1, R1, S2, R2]) Setup(limits *mcts.Limits, nGames uint, nThreads uint) {
	va.NGames = nGames
	va.Limits = limits
	va.NThreads = nThreads
}

func (va *VersusArena[T, P, S1, R1, S2, R2]) Wait() {
	va.wg.Wait()

	for {
		if va.finished.Load() {
			break
		}
		runtime.Gosched()
	}
}

func (va *VersusArena[T, P, S1, R1, S2, R2]) Start(listener ListenerLike[T]) {
	// Start equally distributed work between worker threads
	va.finished.Store(false)
	listener.OnStart()
	nGames := va.NGames / va.NThreads
	rest := uint(0)
	if va.NThreads > 1 {
		rest = va.NGames % va.NThreads
	}
	for i := range va.NThreads {
		delta := 0
		if rest > 0 {
			delta = 1
			rest--
		}
		va.wg.Add(1)

		// Always use a clone, to avoid race conditions when cloning
		p1 := va.Player1.Clone()
		p2 := va.Player2.Clone()
		l := listener.Clone()

		l.SetRow(int(i) + statsRowStart)
		p1.SetLimits(va.Limits)
		p2.SetLimits(va.Limits)
		go va.worker(int(i), int(nGames)+delta, l, p1, p2)
	}
}

func (va *VersusArena[T, P, S1, R1, S2, R2]) worker(id, nGames int, listener ListenerLike[T], p1 ExtMCTS[T, S1, R1, P], p2 ExtMCTS[T, S2, R2, P]) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	var result VersusMatchResult
	var switched bool
	localStats := VersusArenaStats{}
	gamePos := va.Position.Clone()

Loop:
	for i := range nGames {
		if r.Int()%2 == 0 {
			result = playGame(p1, p2, gamePos, listener, id, nGames, i, &localStats, va.ctx)
			switched = false
		} else {
			result = playGame(p2, p1, gamePos, listener, id, nGames, i, &localStats, va.ctx)
			switched = true
		}

		select {
		case <-va.ctx.Done():
			break Loop
		default:
			// continue
		}

		if result == VersusDraw {
			atomic.AddUint32(&va.draws, 1)
			localStats.draws++
		} else {
			if ((result == VersusPl1Win) && !switched) || ((result == VersusPl2Win) && switched) {
				atomic.AddUint32(&va.p1Wins, 1)
				localStats.p1Wins++
			} else {
				atomic.AddUint32(&va.p2Wins, 1)
				localStats.p2Wins++
			}
		}
	}

	va.wg.Done()
	listener.OnFinishedWork(VersusWorkerInfo[T]{
		WorkerID:      id,
		NGames:        nGames,
		FinishedGames: va.Total(),
		P1Wins:        int(localStats.p1Wins),
		P2Wins:        int(localStats.p2Wins),
		Draws:         int(localStats.draws),
	})

	if listener != nil && id == 0 {
		va.wg.Wait()
		listener.Summary(VersusSummaryInfo{
			P1Wins:     va.P1Wins(),
			P2Wins:     va.P2Wins(),
			Draws:      va.Draws(),
			Workers:    int(va.NThreads),
			TotalGames: va.Total(),
		})
		listener.OnEnd()
		va.finished.Store(true)
	}
}

func playGame[T mcts.MoveLike, P PositionLike[T, P], S1 mcts.NodeStatsLike[S1],
	R1 mcts.GameResult, S2 mcts.NodeStatsLike[S2], R2 mcts.GameResult](
	pl1 ExtMCTS[T, S1, R1, P], pl2 ExtMCTS[T, S2, R2, P], gamePos P,
	listener ListenerLike[T], workerId, nGames, finishedGames int,
	versusStats *VersusArenaStats, ctx context.Context,
) VersusMatchResult {
	moves := make([]T, 0, 100)

	if listener != nil {
		listener.OnGameStart()
		defer listener.OnFinishedGame(VersusWorkerInfo[T]{
			WorkerID:      workerId,
			Moves:         moves,
			GameMoveNum:   len(moves),
			NGames:        nGames,
			FinishedGames: finishedGames,
			P1Wins:        versusStats.P1Wins(),
			P2Wins:        versusStats.P2Wins(),
			Draws:         versusStats.Draws(),
		})
	}

	pl1.Reset()
	pl2.Reset()

	pl1.SetPosition(gamePos.Clone())
	pl2.SetPosition(gamePos.Clone())

	var m T
	gameEndedByPl1 := false
	result := VersusDraw
	// Player 1 begins
Loop:
	for !gamePos.IsTerminated() {

		select {
		case <-ctx.Done():
			result = VersusDraw
			break Loop
		default:
			// continue
		}

		m = pl1.Search()
		pl1.MakeMove(m)
		gamePos.MakeMove(m)
		moves = append(moves, m)

		if listener != nil {
			listener.OnMoveMade(VersusWorkerInfo[T]{
				WorkerID:      workerId,
				Moves:         moves,
				GameMoveNum:   len(moves),
				NGames:        nGames,
				FinishedGames: finishedGames,
				P1Wins:        versusStats.P1Wins(),
				P2Wins:        versusStats.P2Wins(),
				Draws:         versusStats.Draws(),
			})
		}

		select {
		case <-ctx.Done():
			result = VersusDraw
			break Loop
		default:
			// continue
		}

		if gamePos.IsTerminated() {
			gameEndedByPl1 = true
			break
		}

		if !pl2.MakeMove(m) {
			pl2.SetPosition(gamePos.Clone())
		}

		m = pl2.Search()
		pl2.MakeMove(m)
		gamePos.MakeMove(m)
		moves = append(moves, m)

		if listener != nil {
			listener.OnMoveMade(VersusWorkerInfo[T]{
				WorkerID:      workerId,
				Moves:         moves,
				GameMoveNum:   len(moves),
				NGames:        nGames,
				FinishedGames: finishedGames,
				P1Wins:        versusStats.P1Wins(),
				P2Wins:        versusStats.P2Wins(),
				Draws:         versusStats.Draws(),
			})
		}

		if !pl1.MakeMove(m) {
			pl1.SetPosition(gamePos.Clone())
		}
	}

	if !gamePos.IsDraw() {
		if gameEndedByPl1 {
			result = VersusPl1Win
		} else {
			result = VersusPl2Win
		}
	}

	// Undo all moves
	for range moves {
		gamePos.Undo()
	}

	return result
}
