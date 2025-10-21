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

type PositionLike[T mcts.MoveLike, P any] interface {
	MakeMove(T)
	Undo()
	IsTerminated() bool
	IsDraw() bool
	Clone() P
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
	p1name   string
	p2name   string
	wg       sync.WaitGroup
	finished atomic.Bool
	ctx      context.Context
}

func NewVersusArena[
	T mcts.MoveLike, P PositionLike[T, P], S1 mcts.NodeStatsLike[S1],
	R1 mcts.GameResult, S2 mcts.NodeStatsLike[S2], R2 mcts.GameResult,
	T1 ExtMCTS[T, S1, R1, P], T2 ExtMCTS[T, S2, R2, P]](
	position P, tree1 T1, tree2 T2,
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

func (va *VersusArena[T, P, S1, R1, S2, R2]) Start(p1name, p2name string, listener ListenerLike[T]) {
	// Start equally distributed work between worker threads
	va.finished.Store(false)
	listener.OnStart()
	nGames := va.NGames / va.NThreads
	rest := uint(0)
	if va.NThreads > 1 {
		rest = va.NGames % va.NThreads
	}
	va.p1name = p1name
	va.p2name = p2name
	va.wg.Add(int(va.NThreads))

	for i := range va.NThreads {
		delta := 0
		if rest > 0 {
			delta = 1
			rest--
		}

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

func (va *VersusArena[T, P, S1, R1, S2, R2]) Results() VersusSummaryInfo {
	return VersusSummaryInfo{
		TotalGames:       va.Total(),
		P1Wins:           va.P1Wins(),
		P2Wins:           va.P2Wins(),
		Draws:            va.Draws(),
		Workers:          int(va.NThreads),
		P1Name:           va.p1name,
		P2Name:           va.p2name,
		FirstToMoveWins:  int(va.firstToMoveWins),
		SecondToMoveWins: int(va.secondToMoveWins),
	}
}

func (va *VersusArena[T, P, S1, R1, S2, R2]) worker(
	id, nGames int,
	listener ListenerLike[T],
	p1 ExtMCTS[T, S1, R1, P],
	p2 ExtMCTS[T, S2, R2, P],
) {
	rng := rand.New(rand.NewSource(
		time.Now().UnixNano() ^ (int64(id) << 32) ^ rand.Int63(),
	))

	localStats := VersusArenaStats{}
	gamePos := va.Position.Clone()

WorkLoop:
	for gameIdx := range nGames {
		p1GoesFirst := (rng.Int()%2 == 0)

		var moves []T
		if p1GoesFirst {
			moves = playGameAndNotify(
				va.ctx, p1, p2, gamePos, listener, id,
				nGames, gameIdx, &localStats, va.p1name, va.p2name, false)
		} else {
			moves = playGameAndNotify(
				va.ctx, p2, p1, gamePos, listener, id,
				nGames, gameIdx, &localStats, va.p2name, va.p1name, true)
		}

		// Check for cancellation
		select {
		case <-va.ctx.Done():
			// Always call paired listeners (OnGameStart and OnFinishedGame)
			if listener != nil {
				listener.OnFinishedGame(
					buildWorkerInfo(
						id, gameIdx+1, nGames, moves,
						&localStats, va.p1name, va.p2name, false))
			}
			break WorkLoop
		default:
		}

		outcome := computeOutcome(gamePos, len(moves))
		agentResult := toAgentResult(outcome, p1GoesFirst)
		va.recordResult(agentResult, outcome.FirstPlayerWon, &localStats)
		undoMoves(gamePos, moves)

		if listener != nil {
			listener.OnFinishedGame(
				buildWorkerInfo(
					id, gameIdx+1, nGames, moves,
					&localStats, va.p1name, va.p2name, false))
		}
	}

	va.wg.Done()

	if listener != nil {
		listener.OnFinishedWork(
			buildWorkerInfo[T](
				id, nGames, va.Total(), nil,
				&localStats, va.p1name, va.p2name, false))
	}

	// Worker 0 waits for all workers and prints summary
	if id == 0 {
		va.wg.Wait()
		if listener != nil {
			listener.Summary(va.Results())
			listener.OnEnd()
		}
		va.finished.Store(true)
	}
}

// recordResult updates both global and local statistics
func (va *VersusArena[T, P, S1, R1, S2, R2]) recordResult(
	agentResult VersusMatchResult,
	firstPlayerWon bool,
	localStats *VersusArenaStats,
) {
	// Update agent win counts
	switch agentResult {
	case VersusPl1Win:
		atomic.AddUint32(&va.p1Wins, 1)
		localStats.p1Wins++
	case VersusPl2Win:
		atomic.AddUint32(&va.p2Wins, 1)
		localStats.p2Wins++
	case VersusDraw:
		atomic.AddUint32(&va.draws, 1)
		localStats.draws++
	}

	// Update first-player advantage stats
	if agentResult != VersusDraw {
		if firstPlayerWon {
			atomic.AddUint32(&va.firstToMoveWins, 1)
			localStats.firstToMoveWins++
		} else {
			atomic.AddUint32(&va.secondToMoveWins, 1)
			localStats.secondToMoveWins++
		}
	}
}

// playGameAndNotify runs a single game with listener callbacks
func playGameAndNotify[
	T mcts.MoveLike,
	P PositionLike[T, P],
	S1 mcts.NodeStatsLike[S1], R1 mcts.GameResult,
	S2 mcts.NodeStatsLike[S2], R2 mcts.GameResult,
](
	ctx context.Context,
	pl1 ExtMCTS[T, S1, R1, P],
	pl2 ExtMCTS[T, S2, R2, P],
	gamePos P,
	listener ListenerLike[T],
	workerID, totalGames, gameIdx int,
	localStats *VersusArenaStats,
	p1Name, p2Name string,
	switched bool,
) []T {
	moves := make([]T, 0, 100)

	if listener != nil {
		listener.OnGameStart()
	}

	pl1.Reset()
	pl2.Reset()
	pl1.SetPosition(gamePos.Clone())
	pl2.SetPosition(gamePos.Clone())

	for !gamePos.IsTerminated() {
		select {
		case <-ctx.Done():
			return moves
		default:
		}

		move := pl1.Search()
		pl1.MakeMove(move)
		gamePos.MakeMove(move)
		moves = append(moves, move)

		if listener != nil {
			listener.OnMoveMade(buildWorkerInfo(
				workerID, gameIdx, totalGames, moves,
				localStats, p1Name, p2Name, switched,
			))
		}

		if gamePos.IsTerminated() {
			return moves
		}

		if !pl2.MakeMove(move) {
			pl2.SetPosition(gamePos.Clone())
		}

		select {
		case <-ctx.Done():
			return moves
		default:
		}

		move = pl2.Search()
		pl2.MakeMove(move)
		gamePos.MakeMove(move)
		moves = append(moves, move)

		if listener != nil {
			listener.OnMoveMade(buildWorkerInfo(
				workerID, gameIdx, totalGames, moves,
				localStats, p1Name, p2Name, switched,
			))
		}

		if !pl1.MakeMove(move) {
			pl1.SetPosition(gamePos.Clone())
		}
	}

	return moves
}

func undoMoves[T mcts.MoveLike, P PositionLike[T, P]](gamePos P, moves []T) {
	for range moves {
		gamePos.Undo()
	}
}

func buildWorkerInfo[T mcts.MoveLike](
	workerID, gameIdx, totalGames int,
	moves []T,
	localStats *VersusArenaStats,
	p1Name, p2Name string,
	switched bool,
) VersusWorkerInfo[T] {
	if switched {
		p1Name, p2Name = p2Name, p1Name
	}

	return VersusWorkerInfo[T]{
		WorkerID:         workerID,
		Moves:            moves,
		GameMoveNum:      len(moves),
		NGames:           totalGames,
		FinishedGames:    gameIdx,
		P1Wins:           int(localStats.p1Wins),
		P2Wins:           int(localStats.p2Wins),
		Draws:            int(localStats.draws),
		FirstToMoveWins:  int(localStats.firstToMoveWins),
		SecondToMoveWins: int(localStats.secondToMoveWins),
		P1Name:           p1Name,
		P2Name:           p2Name,
	}
}
