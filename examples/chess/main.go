package main

/*
Chess MCTS (UCB1) example

This example wires the go-mcts library to a chess engine (dragontoothmg) and
runs a multi-threaded search using the UCB1 selection policy.

What it shows:
- Constructing a chess-specific MCTS instance (UCB1 + default backprop).
- Setting time/thread limits for the search.
- Tuning the UCB exploration constant (c).
- Subscribing a listener to print UCI-like "info ... pv ..." lines and "bestmove ...".
*/

import (
	"fmt"
	"strings"

	"github.com/IlikeChooros/dragontoothmg"
	chessmcts "github.com/IlikeChooros/go-mcts/examples/chess/chess-mcts"
	"github.com/IlikeChooros/go-mcts/pkg/mcts"
)

// MovesToString formats a principal variation (PV) for UCI-style output.
func MovesToString(mvs []dragontoothmg.Move) string {
	moves := make([]string, len(mvs))
	for i := range moves {
		moves[i] = mvs[i].String()
	}
	return strings.Join(moves, " ")
}

func main() {
	// Build a chess-specific MCTS instance configured for:
	// - UCB1 selection policy
	// - NodeStats for per-node visits/outcomes
	// - Default 2-player zero-sum backpropagation
	ucb := chessmcts.NewUcbMcts()

	// Set limits: 8 threads, 2000 ms. Search blocks until done.
	// Tip: set threads to number of logical CPUs for best throughput.
	ucb.SetLimits(mcts.DefaultLimits().SetThreads(8).SetMovetime(2000))

	// UCB exploration constant (c). Higher favors exploration, lower favors exploitation.
	// Typical values are around 0.3 to 1.4 depending on domain; tune empirically.
	// You can also call mcts.SetExplorationParam(c) to clamp to >= 0.
	mcts.ExplorationParam = 0.45

	// Attach a listener to stream live search stats.
	// OnDepth is called periodically; we print UCI-like lines with eval, depth, CPS, PV.
	// OnStop fires once at the end; we repeat the final line and print bestmove.
	listener := mcts.NewStatsListener[dragontoothmg.Move]()
	listener.
		OnDepth(func(lts mcts.ListenerTreeStats[dragontoothmg.Move]) {
			if len(lts.Lines) == 0 {
				return
			}
			main := lts.Lines[0] // principal variation (best line so far)
			fmt.Printf("info eval %.2f depth %d cps %d cycles %d pv %s\n",
				main.Eval, lts.Maxdepth, lts.Cps, lts.Cycles, MovesToString(main.Moves))
		}).
		OnStop(func(lts mcts.ListenerTreeStats[dragontoothmg.Move]) {
			if len(lts.Lines) == 0 {
				return
			}
			fmt.Printf("bestmove %s\n", lts.Lines[0].BestMove.String())
		})

	// Register the listener and run the search.
	ucb.SetListener(listener)
	ucb.Search()
}
