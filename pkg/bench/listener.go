package bench

import (
	"fmt"
	"sync"
	"time"

	"github.com/IlikeChooros/go-mcts/pkg/mcts"
	// For ANSI terminal codes
	"github.com/muesli/termenv"
)

const statsRowStart = 4

var printMutex = sync.Mutex{}

type _ListenerStatus int

const (
	_ListenerStatusStarting _ListenerStatus = iota
	_ListenerStatusOnGoing
	_ListenerStatusFinished
)

type ListenerLike[T mcts.MoveLike] interface {
	OnStart()
	OnEnd()
	Summary(VersusSummaryInfo)
	SetRow(row int)
	OnGameStart()
	OnMoveMade(stats VersusWorkerInfo[T])
	OnFinishedGame(stats VersusWorkerInfo[T])
	OnFinishedWork(stats VersusWorkerInfo[T])
	Clone() ListenerLike[T]
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

func (d DefaultListener[T]) print(stats VersusWorkerInfo[T]) {
	printMutex.Lock()
	defer printMutex.Unlock()

	eta := termenv.String("unknown").Foreground(termenv.ANSIBrightBlack).Italic()
	if d.avgTime != 0 {
		v := d.avgTime * time.Duration(stats.NGames-stats.FinishedGames)
		eta = termenv.String(time.Duration(v).Round(time.Second).String()).Foreground(termenv.ANSIWhite).Italic()
	}

	worker := termenv.String(fmt.Sprintf("Worker %2d", stats.WorkerID)).Foreground(termenv.ANSIColor(33)).Bold()

	var ratios string
	if stats.FinishedGames > 0 {
		ratios = fmt.Sprintf("| %s: %6.2f%% | %s: %6.2f%% | Draw: %6.2f%% |",
			stats.P1Name,
			float64(stats.P1Wins)/float64(stats.FinishedGames)*100,
			stats.P2Name,
			float64(stats.P2Wins)/float64(stats.FinishedGames)*100,
			float64(stats.Draws)/float64(stats.FinishedGames)*100)
	} else {
		ratios = fmt.Sprintf("| %s:      -%% | %s:      -%% | Draw:      -%% |",
			stats.P1Name, stats.P2Name)
	}

	statsLine := termenv.String(fmt.Sprintf("| games %2d/%d | movenum %3d %s eta: %s ",
		stats.FinishedGames, stats.NGames, stats.GameMoveNum, ratios, eta.String()))

	out := termenv.DefaultOutput()
	out.MoveCursor(d.row, 0)
	out.ClearLine()
	out.WriteString(fmt.Sprintf("%s %s", worker.String(), statsLine.String()))
}

func (d *DefaultListener[T]) OnStart() {
	out := termenv.DefaultOutput()
	out.HideCursor()
	out.ClearScreen()
	out.MoveCursor(2, 0)
	title := termenv.String("Versus Arena").Foreground(termenv.ANSIColor(33)).Bold().Underline()
	out.WriteString(fmt.Sprintf("%s\n", title.String()))
	out.WriteString("=====================================\n")
}

func (d *DefaultListener[T]) OnEnd() {
	out := termenv.DefaultOutput()
	out.ShowCursor()
	out.ClearLine()
}

func (d *DefaultListener[T]) Summary(summary VersusSummaryInfo) {
	printMutex.Lock()
	defer printMutex.Unlock()

	out := termenv.DefaultOutput()
	out.MoveCursor(statsRowStart+summary.Workers+1, 0)
	out.ClearLine()
	title := termenv.String("Summary").Foreground(termenv.ANSIColor(33)).Bold().Underline()
	out.WriteString(fmt.Sprintf("%s\n", title.String()))
	out.WriteString("=====================================\n")

	total := summary.TotalGames
	p1WinRate := float64(summary.P1Wins) / float64(total) * 100
	p2WinRate := float64(summary.P2Wins) / float64(total) * 100
	drawRate := float64(summary.Draws) / float64(total) * 100

	out.WriteString(fmt.Sprintf("Total games: %d\n", total))
	out.WriteString(fmt.Sprintf("%s: %d (%.2f%%)\n", summary.P1Name, summary.P1Wins, p1WinRate))
	out.WriteString(fmt.Sprintf("%s: %d (%.2f%%)\n", summary.P2Name, summary.P2Wins, p2WinRate))
	out.WriteString(fmt.Sprintf("Draws: %d (%.2f%%)\n", summary.Draws, drawRate))
	out.WriteString("=====================================\n")
}

func (d *DefaultListener[T]) OnGameStart() {
	d.gameStartTime = time.Now()
}

func (d DefaultListener[T]) OnMoveMade(stats VersusWorkerInfo[T]) {
	d.print(stats)
}

func (d *DefaultListener[T]) OnFinishedGame(stats VersusWorkerInfo[T]) {
	d.avgTime = time.Since(d.gameStartTime)
	d.print(stats)
}

func (d DefaultListener[T]) OnFinishedWork(stats VersusWorkerInfo[T]) {
	printMutex.Lock()
	defer printMutex.Unlock()

	out := termenv.DefaultOutput()
	out.MoveCursor(d.row, 0)
	out.ClearLine()
	worker := termenv.String(fmt.Sprintf("Worker %d", stats.WorkerID)).Foreground(termenv.ANSIColor(33)).Bold()
	status := termenv.String(_ListenerStatusFinished.String()).Foreground(termenv.ANSIColor(34)).Bold()

	out.WriteString(fmt.Sprintf("%s %s\n", worker.String(), status.String()))
}

func (d *DefaultListener[T]) SetRow(n int) {
	d.row = n
}

func (d *DefaultListener[T]) Clone() ListenerLike[T] {
	return &DefaultListener[T]{row: d.row, ansiLinePos: d.ansiLinePos, gameStartTime: d.gameStartTime, avgTime: d.avgTime}
}
