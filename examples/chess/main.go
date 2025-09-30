package main

import (
	"fmt"
	"strings"

	"github.com/IlikeChooros/dragontoothmg"
	chessmcts "github.com/IlikeChooros/go-mcts/examples/chess/chess-mcts"
	"github.com/IlikeChooros/go-mcts/pkg/mcts"
)

func MovesToString(mvs []dragontoothmg.Move) string {
	moves := make([]string, len(mvs))
	for i := range moves {
		moves[i] = mvs[i].String()
	}
	return strings.Join(moves, " ")
}

func main() {
	ucb := chessmcts.NewUcbMcts()
	ucb.SetLimits(mcts.DefaultLimits().SetThreads(8).SetMovetime(2000))
	mcts.ExplorationParam = 0.45

	listener := mcts.NewStatsListener[dragontoothmg.Move]()
	listener.
		OnDepth(func(lts mcts.ListenerTreeStats[dragontoothmg.Move]) {
			if len(lts.Lines) == 0 {
				return
			}
			main := lts.Lines[0]
			fmt.Printf("info eval %.2f depth %d cps %d cycles %d pv %s\n",
				main.Eval, lts.Maxdepth, lts.Cps, lts.Cycles, MovesToString(main.Moves))
		}).
		OnStop(func(lts mcts.ListenerTreeStats[dragontoothmg.Move]) {
			if len(lts.Lines) == 0 {
				return
			}
			main := lts.Lines[0]
			fmt.Printf("info eval %.2f depth %d cps %d cycles %d pv %s\n",
				main.Eval, lts.Maxdepth, lts.Cps, lts.Cycles, MovesToString(main.Moves))
			fmt.Printf("bestmove %s\n", main.BestMove.String())
		})

	ucb.SetListener(listener)
	ucb.Search()
}
