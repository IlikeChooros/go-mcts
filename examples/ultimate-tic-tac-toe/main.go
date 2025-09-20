package main

/*

This is an example of ultimate tic tac toe implementation.
If you don't know the rules, see: https://en.wikipedia.org/wiki/Ultimate_tic-tac-toe

All of the game logic is in the 'uttt' package (examples/ultimate-tic-tac-toe/uttt),
including the MCTS implementation (uttt_mcts.go).

Example below shows how to run a search and get the result.

*/

import (
	"fmt"

	uttt "go-mcts/examples/ultimate-tic-tac-toe/uttt"
	mcts "go-mcts/pkg/mcts"
)

func main() {
	fmt.Println("Ultimate Tic Tac Toe MCTS Example")

	// Create a new UTTT MCTS instance
	tree := uttt.NewUtttMCTS(*uttt.NewPosition())

	// Set search parameters
	tree.SetLimits(mcts.DefaultLimits().SetMovetime(2000).SetThreads(2))

	// Run the search, will block until done
	tree.Search()

	// Get the search result: pv, best move, winrate, visits, etc
	result := tree.SearchResult(mcts.BestChildWinRate)

	fmt.Println("result: ", result.String())
	fmt.Printf("Tree size: %d\n", tree.Size())
	fmt.Printf("Memory used: %.2f MB\n", float32(tree.MemoryUsage())/1024.0/1024.0)
	fmt.Printf("Collisions: %.2f%% (%d)\n", tree.CollisionFactor()*100.0, tree.CollisionCount())
}
