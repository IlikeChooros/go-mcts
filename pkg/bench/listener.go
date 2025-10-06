package bench

import (
	"fmt"
	"time"

	"github.com/IlikeChooros/go-mcts/pkg/mcts"
)

// ANSI codes
const (
	ANSI_CLEAR_SCREEN             = "\033[2J"
	ANSI_CLEAR_SCREEN_FROM_CURSOR = "\033[J"
	ANSI_CLEAR_SCREEN_TO_CURSOR   = "\033[1J"

	ANSI_CLEAR_LINE             = "\033[2K"
	ANSI_CLEAR_LINE_FROM_CURSOR = "\033[K"
	ANSI_CLEAR_LINE_TO_CURSOR   = "\033[1K"

	ANSI_BG_BLACK = "\033[40m"
	ANSI_BG_RED   = "\033[41m"

	ANSI_CURSOR_HIDE = "\033[?25l"
	ANSI_CURSOR_SHOW = "\033[?25h"

	ANSI_CURSOR_UP   = "\033[%dB"
	ANSI_CURSOR_DOWN = "\033[%dC"
	ANSI_CURSOR_POS  = "\033[%d;%dH"
)

type _ListenerStatus int

const (
	_ListenerStatusStarting _ListenerStatus = iota
	_ListenerStatusOnGoing
	_ListenerStatusFinished
)

type ListenerStats[T mcts.MoveLike] struct {
	NGames        int
	FinishedGames int
	GameMoveNum   int
	Moves         []T
}

type ListenerLike[T mcts.MoveLike] interface {
	SetRow(row int)
	OnGameStart()
	OnMoveMade(stats ListenerStats[T])
	OnFinishedGame(stats ListenerStats[T])
	OnFinishedWork()
}

type DefaultListener[T mcts.MoveLike] struct {
	ansiLinePos   string
	gameStartTime time.Time
	avgTime       time.Duration
	row           int
}

func (s _ListenerStatus) String() string {
	switch s {
	case _ListenerStatusFinished:
		return "finished"
	case _ListenerStatusOnGoing:
		return "ongoing"
	default:
		return "starting"
	}
}

func (d DefaultListener[T]) print(stats ListenerStats[T]) {

	var etaStr string
	if d.avgTime == 0 {
		eta := d.avgTime.Milliseconds() * (int64(stats.NGames - stats.FinishedGames))
		etaStr = time.Duration(eta).String()
	} else {
		etaStr = "unknown"
	}

	fmt.Printf("%sStatus: %s %d/%d games %d movenum eta: %s %s",
		d.ansiLinePos, _ListenerStatusOnGoing.String(), stats.FinishedGames, stats.NGames, stats.GameMoveNum, etaStr,
		ANSI_CLEAR_LINE_FROM_CURSOR)
}

func (d *DefaultListener[T]) OnGameStart() {
	d.gameStartTime = time.Now()
}

func (d DefaultListener[T]) OnMoveMade(stats ListenerStats[T]) {
	d.print(stats)
}

func (d *DefaultListener[T]) OnFinishedGame(stats ListenerStats[T]) {
	d.avgTime = time.Since(d.gameStartTime)
	d.print(stats)
}

func (d DefaultListener[T]) OnFinishedWork() {
	fmt.Printf("%sStatus: %s%s", d.ansiLinePos, _ListenerStatusFinished.String(), ANSI_CLEAR_LINE_FROM_CURSOR)
}

func (d *DefaultListener[T]) SetRow(n int) {
	d.row = n
	d.ansiLinePos = fmt.Sprintf(ANSI_CURSOR_POS, d.row, 0)
}
