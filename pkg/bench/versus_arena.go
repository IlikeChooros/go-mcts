package bench

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IlikeChooros/go-mcts/pkg/mcts"
)

/*
Arena benchmark subpackage, allows to play a series of games between two
different MCTS configurations.
*/

type VersusMatchResult int

const (
	VersusPl1Win VersusMatchResult = 1
	VersusPl2Win VersusMatchResult = -1
	VersusDraw   VersusMatchResult = 0
)

type PositionLike[T mcts.MoveLike] interface {
	Make(T)
	Undo()
	IsTerminal() bool
	IsDraw() bool
}

type VersusArenaStats struct {
	p1Wins atomic.Uint32
	p2Wins atomic.Uint32
	draws  atomic.Uint32
}

func (vas *VersusArenaStats) Total() int {
	return int(vas.p1Wins.Load() + vas.p2Wins.Load() + vas.draws.Load())
}

func (vas *VersusArenaStats) P1Wins() int {
	return int(vas.p1Wins.Load())
}

func (vas *VersusArenaStats) P2Wins() int {
	return int(vas.p2Wins.Load())
}

func (vas *VersusArenaStats) Draws() int {
	return int(vas.draws.Load())
}

type VersusWorkerInfo[T mcts.MoveLike] struct {
	NGames        int
	FinishedGames int
	GameMoveNum   int
	Moves         []T
}

type ExtMCTS[T mcts.MoveLike, S mcts.NodeStatsLike, R mcts.GameResult] interface {
	Reset()          // reset the tree, keeping the ops and limits
	MakeMove(T) bool // make a move in the root position, updating the tree
	SetLimits(*mcts.Limits)
	Search() T
	SetPosition(PositionLike[T])
	Clone() ExtMCTS[T, S, R]
}

type VersusArena[T mcts.MoveLike, S1 mcts.NodeStatsLike, R1 mcts.GameResult, S2 mcts.NodeStatsLike, R2 mcts.GameResult] struct {
	VersusArenaStats
	Player1  ExtMCTS[T, S1, R1]
	Player2  ExtMCTS[T, S2, R2]
	NGames   uint
	NThreads uint
	Limits   *mcts.Limits
	Position PositionLike[T]
	wg       sync.WaitGroup
}

func NewVersusArena[
	T mcts.MoveLike, P PositionLike[T], S1 mcts.NodeStatsLike,
	R1 mcts.GameResult, S2 mcts.NodeStatsLike, R2 mcts.GameResult](
	position P, tree1 ExtMCTS[T, S1, R1], tree2 ExtMCTS[T, S2, R2],
) *VersusArena[T, S1, R1, S2, R2] {
	return &VersusArena[T, S1, R1, S2, R2]{
		Player1:  tree1,
		Player2:  tree2,
		NGames:   100,
		NThreads: 2,
		Limits:   mcts.DefaultLimits().SetMovetime(1000),
		Position: position,
	}
}

func (va *VersusArena[T, S1, R1, S2, R2]) Setup(limits *mcts.Limits, nGames uint, nThreads uint) {
	va.NGames = nGames
	va.Limits = limits
	va.NThreads = nThreads
}

func (va *VersusArena[T, S1, R1, S2, R2]) Wait() {
	va.wg.Wait()
}

func (va *VersusArena[T, S1, R1, S2, R2]) Start(listener ListenerLike[T]) {
	// Start equally distributed work between worker threads
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
		p1 := va.Player1
		p2 := va.Player2

		if i > 0 {
			p1 = va.Player1.Clone()
			p2 = va.Player2.Clone()
		}

		go va.worker(int(nGames)+delta, listener, p1, p2)
	}
}

func (va *VersusArena[T, S1, R1, S2, R2]) worker(nGames int, listener ListenerLike[T], p1 ExtMCTS[T, S1, R1], p2 ExtMCTS[T, S2, R2]) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	var result VersusMatchResult
	var switched bool

	for range nGames {
		var list ListenerLike[T] = nil
		if listener != nil {
			list = listener.Clone()
		}
		if r.Int()%2 == 0 {
			result = playGame(p1, p2, va.Position, list, nGames, va.Total())
			switched = false
		} else {
			result = playGame(p2, p1, va.Position, list, nGames, va.Total())
			switched = true
		}

		if result == VersusDraw {
			va.draws.Add(1)
		} else {
			if ((result == VersusPl1Win) && !switched) || ((result == VersusPl2Win) && switched) {
				va.p1Wins.Add(1)
			} else {
				va.p2Wins.Add(1)
			}
		}
	}

	va.wg.Done()
}

func playGame[T mcts.MoveLike, S1 mcts.NodeStatsLike,
	R1 mcts.GameResult, S2 mcts.NodeStatsLike, R2 mcts.GameResult](
	pl1 ExtMCTS[T, S1, R1], pl2 ExtMCTS[T, S2, R2], p PositionLike[T],
	listener ListenerLike[T], gameNum, finishedGames int,
) VersusMatchResult {
	if listener != nil {
		listener.OnGameStart()
		defer listener.OnFinishedWork()
	}

	pl1.Reset()
	pl2.Reset()

	pl1.SetPosition(p)
	pl2.SetPosition(p)

	moves := make([]T, 0, 100)
	var m T
	gameEndedByPl1 := false
	result := VersusDraw
	// Player 1 begins
	for !p.IsTerminal() {

		m = pl1.Search()
		pl1.MakeMove(m)
		p.Make(m)

		if listener != nil {
			moves = append(moves, m)
			listener.OnMoveMade(VersusWorkerInfo[T]{
				Moves:         moves,
				GameMoveNum:   len(moves),
				NGames:        gameNum,
				FinishedGames: finishedGames,
			})
		}

		if !pl2.MakeMove(m) {
			pl2.SetPosition(p)
		}

		if p.IsTerminal() {
			gameEndedByPl1 = true
			break
		}

		m = pl2.Search()
		pl2.MakeMove(m)
		p.Make(m)

		if listener != nil {
			moves = append(moves, m)
			listener.OnMoveMade(VersusWorkerInfo[T]{
				Moves:         moves,
				GameMoveNum:   len(moves),
				NGames:        gameNum,
				FinishedGames: finishedGames,
			})
		}

		if !pl1.MakeMove(m) {
			pl1.SetPosition(p)
		}
	}

	if !p.IsDraw() {
		if gameEndedByPl1 {
			result = VersusPl1Win
		} else {
			result = VersusPl2Win
		}
	}

	return result
}
