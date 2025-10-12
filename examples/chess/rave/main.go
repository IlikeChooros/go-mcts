package main

/*
Chess MCTS (RAVE) example

This example wires the go-mcts library to a chess engine (dragontoothmg) and
runs a search with the RAVE (AMAF) selection policy. It also demonstrates:

- Customizing the RAVE beta function (how much AMAF influences selection).
- Setting search limits (time and threads).
- Subscribing a listener for live depth and PV updates, printing UCI-like lines.

To keep things simple, all chess rules, move generation and make/undo are provided
by dragontoothmg. The example focuses on configuring and running the MCTS.
*/

import (
	"fmt"
	"strings"

	"github.com/IlikeChooros/dragontoothmg"
	chessmcts "github.com/IlikeChooros/go-mcts/examples/chess/chess-mcts"
	"github.com/IlikeChooros/go-mcts/pkg/mcts"
)

// MovesToString formats a sequence of moves for pretty printing of PV lines.
func MovesToString(mvs []dragontoothmg.Move) string {
	moves := make([]string, len(mvs))
	for i := range moves {
		moves[i] = mvs[i].String()
	}
	return strings.Join(moves, " ")
}

func main() {
	// Construct a chess-specific MCTS instance that uses:
	// - RAVE selection
	// - RaveStats node stats (AMAF counters)
	// - RaveBackprop for AMAF updates
	rave := chessmcts.NewRaveMcts()

	// Customize the RAVE beta function (influence of AMAF vs. standard Q).
	// Smaller b makes AMAF decay faster as visits grow.
	// This is a variant of the function discussed by D. Silver.
	rave.Strategy().SetBetaFunction(func(n, n_rave int32) float64 {
		// const (
		// b      = 0.1       // controls the AMAF weight decay
		// factor = 4 * b * b // smoothing factor
		// )
		// return float64(n) / (float64(n+n_rave) + factor*float64(n*n_rave))

		// Alternative: linear decay until a threshold, then 0.
		const threshold = 10000
		if n >= threshold {
			return 0
		}
		return float64(threshold-n) / float64(threshold)
	})
	rave.Strategy().SetExplorationParam(0.2) // UCB exploration parameter

	// Set search limits: 8 threads, 2000 ms. The search call blocks until done.
	rave.SetLimits(mcts.DefaultLimits().SetThreads(4).SetCycles(60000))

	// Attach a stats listener to stream live search information.
	// We print UCI-style info lines on depth updates and final bestmove on stop.
	listener := mcts.NewStatsListener[dragontoothmg.Move]()
	listener.
		OnDepth(func(lts mcts.ListenerTreeStats[dragontoothmg.Move]) {
			fmt.Printf("info eval %.2f depth %d cps %d cycles %d pv %s\n",
				lts.Lines[0].Eval, lts.Maxdepth, lts.Cps, lts.Cycles, MovesToString(lts.Lines[0].Moves))
		}).
		OnStop(func(lts mcts.ListenerTreeStats[dragontoothmg.Move]) {
			fmt.Printf("info eval %.2f depth %d cps %d cycles %d pv %s\n",
				lts.Lines[0].Eval, lts.Maxdepth, lts.Cps, lts.Cycles, MovesToString(lts.Lines[0].Moves))
			fmt.Printf("bestmove %s\n", lts.Lines[0].BestMove.String())
		})

	// Register listener and run the search.
	rave.SetListener(listener)
	rave.Search()
}
