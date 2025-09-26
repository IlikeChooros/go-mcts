package main

/*

This is an example usage of RAVE selection with MCTS,
the actual implementation is in 'examples/ultimate-tic-tac-toe/uttt/rave/uttt_mcts.go'

There is no difference to the MCTS API, even the listener has the same type as in the UCB example.
There is however a visible slowdown in the search speed, because during the backpropagation many nodes are updated.


*/

import (
	"fmt"

	uttt "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/core"
	rave "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/rave"
	"github.com/IlikeChooros/go-mcts/pkg/mcts"
)

func main() {
	const (
		bestChildPolicy = mcts.BestChildMostVisits
		cycleInterval   = 50000
	)

	fmt.Println("Ultimate Tic Tac Toe MCTS RAVE Example")

	// Create a new UTTT MCTS instance
	tree := rave.NewUtttMCTS(*uttt.NewPosition())

	// Set search parameters
	tree.SetLimits(mcts.DefaultLimits().SetMovetime(2000).SetThreads(4))

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
			fmt.Printf("[Depth %d] %s\n",
				stats.Maxdepth, result.String())
		}).
		OnCycle(func(stats mcts.ListenerTreeStats[uttt.PosType]) {
			result := tree.SearchResult(bestChildPolicy)
			fmt.Printf("[Cycle %d] %s\n", stats.Cycles, result.String())
		}).
		OnStop(func(stats mcts.ListenerTreeStats[uttt.PosType]) {
			// Now the 'StopReason' is available
			fmt.Printf("Search stopped, reason: %s\n", stats.StopReason.String())
		}).
		SetCycleInterval(cycleInterval) // Call every 50000 cycles, failing to set will make the listener call on every cycle

	// Attach the listener to the MCTS tree
	tree.SetListener(listener)

	// Run the search, will block until done
	tree.Search()

	// Get the search result: pv, best move, winrate, visits, etc
	result := tree.SearchResult(mcts.BestChildWinRate)

	fmt.Println("result: ", result.String())
	fmt.Printf("Tree size: %d\n", tree.Size())
	fmt.Printf("Memory used: %.2f MB\n", float32(tree.MemoryUsage())/1024.0/1024.0)
	fmt.Printf("Collisions: %.2f%% (%d)\n", tree.CollisionFactor()*100.0, tree.CollisionCount())
}
