package main

/*
This example shows how to get real-time updates from the MCTS[T, S, R, O, A]search, by using
the built-in Listener.

The listener has 3 methods:
- OnDepth: called when a new maximum depth is reached
- OnCycle: called every N cycles (N is configurable)
- OnStop: called when the search is finished

Below, there is implemented a simple listener that prints the current best move
and evaluation every time a new depth is reached or every 50000 cycles.

*/

import (
	"fmt"

	uttt "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/core"
	basic_uttt_mcts "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/ucb"
	mcts "github.com/IlikeChooros/go-mcts/pkg/mcts"
)

func main() {
	fmt.Println("Ultimate Tic Tac Toe MCTS[T, S, R, O, A]Real-Time Listener Example")

	const (
		// How many cycles to run before calling the OnCycle listener
		cycleInterval   = 50000
		bestChildPolicy = mcts.BestChildMostVisits
	)

	// Create a new UTTT MCTS[T, S, R, O, A]instance
	position := uttt.NewPosition()
	// turn := position.Turn()
	tree := basic_uttt_mcts.NewUtttMCTS(*position)

	// Set search parameters, try using different limits and see how it affects the search
	// Also notice that MaxDepth != Pv depth, MaxDepth is only the maximum depth reached in the tree,
	// meaning there is *usually* only 1 node at that depth, so the Pv might not include that depth at all
	tree.SetLimits(mcts.DefaultLimits().SetThreads(4).SetMbSize(32).SetDepth(8).SetMultiPv(2))

	// Create a new listener, this shouldn't be a pointer, as it will be copied internally
	listener := mcts.NewStatsListener[uttt.PosType]()

	// Set the listener to print the current best move and evaluation on depth change
	// OnDepth: will be called only by the main search thread, so no need for synchronization
	// OnCycle: will be called every N cycles (SetCycleInterval to set N), this might slow down the search significantly if N is small,
	//          because of pv evaluation, so use it wisely
	// OnStop:  will be called once, when the search ends, making the 'StopReason' available
	listener.
		OnDepth(func(stats mcts.ListenerTreeStats[uttt.PosType]) {
			// Get the current best move and evaluation, using the 'MostVisits' policy
			result := tree.SearchResult(bestChildPolicy)
			fmt.Printf("[Depth %d] %s\n", stats.Maxdepth, result.String())
		}).
		OnCycle(func(stats mcts.ListenerTreeStats[uttt.PosType]) {
			result := tree.SearchResult(bestChildPolicy)
			fmt.Printf("[Cycles %d] %s\n", stats.Cycles, result.String())
		}).
		OnStop(func(stats mcts.ListenerTreeStats[uttt.PosType]) {
			// Now the 'StopReason' is available
			fmt.Printf("Search stopped, reason: %s\n", stats.StopReason.String())
		}).
		SetCycleInterval(cycleInterval) // Call every 50000 cycles, failing to set will make the listener call on every cycle

	// Attach the listener to the MCTS[T, S, R, O, A]tree
	tree.SetListener(listener)

	// Run the search, will block until done
	tree.Search()

	fmt.Printf("Used memory: %.2fMB\n", float32(tree.MemoryUsage())/1024.0/1024.0)
	fmt.Print("Search finished, running another search with different limits...\n\n")

	// To continue the search, update the limits and call Search() again
	// (since memory limit will terminate the search immediately)
	// Or discard the current search and reset the tree to the current position
	tree.SetLimits(mcts.DefaultLimits().SetMovetime(2000).SetThreads(8))
	// Try commenting the line below
	tree.Reset()
	tree.Search()

	// Get the final search result
	result := tree.SearchResult(bestChildPolicy)

	fmt.Println("Final result: ", result.String())
	fmt.Printf("Used memory: %.2fMB\n", float32(tree.MemoryUsage())/1024.0/1024.0)
}
