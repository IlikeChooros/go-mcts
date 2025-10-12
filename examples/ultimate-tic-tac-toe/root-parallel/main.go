package main

/*

This example shows the relative speed up of the search using
both tree and root parallel multithreading policies.

*/

import (
	"fmt"

	uttt "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/core"
	ucb "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/ucb"
	mcts "github.com/IlikeChooros/go-mcts/pkg/mcts"
)

type SearchStats struct {
	Cps        []int
	Depth      []int
	PvLen      []int
	Colls      []float64
	RootVisits []int32
}

func NewSearchStats(maxthreads int) *SearchStats {
	return &SearchStats{
		Cps:        make([]int, maxthreads),
		Depth:      make([]int, maxthreads),
		PvLen:      make([]int, maxthreads),
		Colls:      make([]float64, maxthreads),
		RootVisits: make([]int32, maxthreads),
	}
}

func (s *SearchStats) Set(i, cps, depth, pvlen int, collfactor float64, rootVisits int32) {
	s.Cps[i] = cps
	s.Depth[i] = depth
	s.PvLen[i] = pvlen
	s.Colls[i] = collfactor
	s.RootVisits[i] = rootVisits
}

func Summary(nthreads int, tree, root *SearchStats) {
	fmt.Println("Summary")
	for i := range nthreads {
		fmt.Printf("Threads: %d (tree, root)\n", i+1)
		fmt.Printf("\tDepth: %12d - %d\n", tree.Depth[i], root.Depth[i])
		fmt.Printf("\tCps: %14d - %d\n", tree.Cps[i], root.Cps[i])
		fmt.Printf("\tPvLen: %12d - %d\n", tree.PvLen[i], root.PvLen[i])
		fmt.Printf("\tColls: %11.2f%% - %.2f%%\n", tree.Colls[i]*100.0, root.Colls[i]*100)
		fmt.Printf("\tRootVisits: %7d - %d\n", tree.RootVisits[i], root.RootVisits[i])
		fmt.Printf("\tSpeedup: %10.2f - %.2f\n", float64(tree.Cps[i])/float64(tree.Cps[0]),
			float64(root.Cps[i])/float64(root.Cps[0]))
	}
	fmt.Println()
}

func main() {
	fmt.Println("Ultimate Tic Tac Toe MCTS Example")

	// Create a new UTTT MCTS instance
	tree := ucb.NewUtttMCTS(*uttt.NewPosition())

	// Lets see how much of a difference makes the multithread policy
	// We will be comparing the max depth, cycles per second, speed up ratio
	// and collision fators
	const (
		MaxThreads      = 4
		bestChildPolicy = mcts.BestChildMostVisits
	)

	rootParallelStats := NewSearchStats(MaxThreads)
	treeParallelStats := NewSearchStats(MaxThreads)

	for i := range MaxThreads {
		threads := i + 1
		fmt.Printf("Running search with %d threads...\n", threads)

		// Discard current search tree
		tree.Reset()
		tree.SetLimits(mcts.DefaultLimits().SetMovetime(400).SetThreads(threads))

		// Root-parallel has better scaling, so we are expecting
		// the ratio to be close to the number of threads
		tree.SetMultithreadPolicy(mcts.MultithreadRootParallel)
		tree.Search()
		res := tree.SearchResult(bestChildPolicy)
		rootParallelStats.Set(i, int(res.Cps), res.Depth, len(res.Lines[0].Pv), tree.CollisionFactor(), tree.Root.Stats.N())
		fmt.Printf("Root parallel: %s\n", res.String())

		// Discard current search tree
		tree.Reset()

		// Run again
		tree.SetMultithreadPolicy(mcts.MultithreadTreeParallel)
		tree.Search()
		res = tree.SearchResult(bestChildPolicy)
		treeParallelStats.Set(i, int(res.Cps), res.Depth, len(res.Lines[0].Pv), tree.CollisionFactor(), tree.Root.Stats.N())
		fmt.Printf("Tree parallel: %s\n", res.String())
	}

	// Compare the results
	Summary(MaxThreads, treeParallelStats, rootParallelStats)
}
