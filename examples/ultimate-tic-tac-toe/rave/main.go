package main

import (
	"fmt"
	uttt "go-mcts/examples/ultimate-tic-tac-toe/uttt/core"
	rave "go-mcts/examples/ultimate-tic-tac-toe/uttt/rave"
	"go-mcts/pkg/mcts"
)

func main() {
	fmt.Println("Ultimate Tic Tac Toe MCTS Example")

	// Create a new UTTT MCTS instance
	tree := rave.NewUtttMCTS(*uttt.NewPosition())

	// Set search parameters
	tree.SetLimits(mcts.DefaultLimits().SetMovetime(2000).SetThreads(4))

	// Run the search, will block until done
	tree.Search()

	// Get the search result: pv, best move, winrate, visits, etc
	result := tree.SearchResult(mcts.BestChildWinRate)

	fmt.Println("result: ", result.String())
	fmt.Printf("Tree size: %d\n", tree.Size())
	fmt.Printf("Memory used: %.2f MB\n", float32(tree.MemoryUsage())/1024.0/1024.0)
	fmt.Printf("Collisions: %.2f%% (%d)\n", tree.CollisionFactor()*100.0, tree.CollisionCount())
}
