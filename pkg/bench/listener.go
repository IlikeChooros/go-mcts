package bench

import "github.com/IlikeChooros/go-mcts/pkg/mcts"

// ANSI codes
const (
	ANSI_CLEAR_SCREEN             = "\033[2J"
	ANSI_CLEAR_SCREEN_FROM_CURSOR = "\033[J"
	ANSI_CLEAR_SCREEN_TO_CURSOR   = "\033[1J"
	ANSI_CLEAR_LINE               = "\033[2K"

	ANSI_BG_BLACK = "\033[40m"
	ANSI_BG_RED   = "\033[41m"

	ANSI_CURSOR_HIDE = "\033[?25l"
	ANSI_CURSOR_SHOW = "\033[?25h"

	ANSI_CURSOR_UP   = "\033[%iB"
	ANSI_CURSOR_DOWN = "\033[%iC"
)

type ListenerStats[T mcts.MoveLike] struct {
	NGames        int
	FinishedGames int
	GameMoveNum   int
	Moves         []T
}

type ListenerLike[T mcts.MoveLike] interface {
	SetLineNum(row int)
	OnMoveMade(stats ListenerStats[T])
	OnFinishedGame(stats ListenerStats[T])
	OnFinishedWork()
}

type DefaultListener[T mcts.MoveLike] struct {
	row int
}

func (d DefaultListener[T]) print(stats ListenerStats[T]) {

}

func (d DefaultListener[T]) OnMoveMade(stats ListenerStats[T]) {

}

func (d DefaultListener[T]) OnFinishedGame(stats ListenerStats[T]) {

}

func (d DefaultListener[T]) OnFinishedWork() {

}
