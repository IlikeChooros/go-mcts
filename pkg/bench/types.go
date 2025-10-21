package bench

import (
	"sync/atomic"

	"github.com/IlikeChooros/go-mcts/pkg/mcts"
)

type VersusMatchResult int

const (
	VersusPl1Win VersusMatchResult = 1
	VersusPl2Win VersusMatchResult = -1
	VersusDraw   VersusMatchResult = 0
)

type VersusArenaStats struct {
	p1Wins           uint32
	p2Wins           uint32
	draws            uint32
	firstToMoveWins  uint32
	secondToMoveWins uint32
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

func (vas *VersusArenaStats) FirstToMoveWins() int {
	return int(atomic.LoadUint32(&vas.firstToMoveWins))
}

func (vas *VersusArenaStats) SecondToMoveWins() int {
	return int(atomic.LoadUint32(&vas.secondToMoveWins))
}

type VersusWorkerInfo[T mcts.MoveLike] struct {
	WorkerID         int
	NGames           int
	FinishedGames    int
	GameMoveNum      int
	Moves            []T
	P1Wins           int
	P2Wins           int
	Draws            int
	FirstToMoveWins  int
	SecondToMoveWins int
	P1Name           string
	P2Name           string
}

type VersusSummaryInfo struct {
	TotalGames       int    `json:"total_games"`
	P1Wins           int    `json:"player1_wins"`
	P2Wins           int    `json:"player2_wins"`
	FirstToMoveWins  int    `json:"first_to_move_wins"`
	SecondToMoveWins int    `json:"second_to_move_wins"`
	Draws            int    `json:"draws"`
	Workers          int    `json:"workers"`
	P1Name           string `json:"player1_name"`
	P2Name           string `json:"player2_name"`
}

// represents result from the first-player's perspective in a single game
type GameOutcome struct {
	FirstPlayerWon bool
	IsDraw         bool
}

// maps a game outcome to which agent won, given player assignments
func toAgentResult(outcome GameOutcome, p1WentFirst bool) VersusMatchResult {
	if outcome.IsDraw {
		return VersusDraw
	}

	if p1WentFirst == outcome.FirstPlayerWon {
		return VersusPl1Win
	}
	return VersusPl2Win
}

// determines winner based on game state and move history
func computeOutcome[T mcts.MoveLike, P PositionLike[T, P]](
	gamePos P,
	moveCount int,
) GameOutcome {
	if !gamePos.IsTerminated() {
		panic("computeOutcome: position not terminated")
	}

	if gamePos.IsDraw() {
		return GameOutcome{IsDraw: true}
	}

	firstPlayerWon := (moveCount%2 == 1)
	return GameOutcome{FirstPlayerWon: firstPlayerWon}
}
