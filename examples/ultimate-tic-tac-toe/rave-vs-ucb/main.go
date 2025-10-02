package main

/*

This is an example usage of RAVE selection with MCTS,
the actual implementation is in 'examples/ultimate-tic-tac-toe/uttt/rave/uttt_mcts.go'

There is no difference to the MCTS API, even the listener has the same type as in the UCB example.
There is however a visible slowdown in the search speed, because during the backpropagation many nodes are updated.


*/

import (
	"fmt"
	"math"

	uttt "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/core"
	rave "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/rave"
	ucb "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/ucb"
	"github.com/IlikeChooros/go-mcts/pkg/mcts"
)

type SearchStats struct {
	Cps       []int
	Depth     []int
	PvLen     []int
	Colls     []float64
	AvgVisits []float64
}

func NewSearchStats(maxthreads int) *SearchStats {
	return &SearchStats{
		Cps:   make([]int, maxthreads),
		Depth: make([]int, maxthreads),
		PvLen: make([]int, maxthreads),
		Colls: make([]float64, maxthreads),
	}
}

func (s *SearchStats) Set(i, cps, depth, pvlen int, collfactor float64) {
	s.Cps[i] = cps
	s.Depth[i] = depth
	s.PvLen[i] = pvlen
	s.Colls[i] = collfactor
}

func Summary(nthreads int, ucb, rave *SearchStats) {
	fmt.Println("Summary")
	for i := range nthreads {
		fmt.Printf("Threads: %d (ucb, rave)\n", i+1)
		fmt.Printf("\tDepth: %12d - %d\n", ucb.Depth[i], rave.Depth[i])
		fmt.Printf("\tCps: %14d - %d\n", ucb.Cps[i], rave.Cps[i])
		fmt.Printf("\tPvLen: %12d - %d\n", ucb.PvLen[i], rave.PvLen[i])
		fmt.Printf("\tColls: %11.2f%% - %.2f%%\n", ucb.Colls[i]*100.0, rave.Colls[i]*100)
		fmt.Printf("\tSpeedup: %10.2f - %.2f\n", float64(ucb.Cps[i])/float64(ucb.Cps[0]),
			float64(rave.Cps[i])/float64(rave.Cps[0]))
	}
	fmt.Println()
}

func main() {
	fmt.Println("Ultimate Tic Tac Toe MCTS Example")

	// Create a new UTTT MCTS instance
	ucbTree := ucb.NewUtttMCTS(*uttt.NewPosition())
	raveTree := rave.NewUtttMCTS(*uttt.NewPosition())

	// Lets see how much of a difference makes the multithread policy
	// We will be comparing the max depth, cycles per second, speed up ratio
	// and collision fators
	const (
		MaxThreads      = 4
		bestChildPolicy = mcts.BestChildMostVisits
		nCycles         = 120000
	)
	mcts.SetRaveBetaFunction(func(playouts, playoutsContatingMove int32) float64 {
		const K = 750.0
		return math.Sqrt(K / (3.0*float64(playouts) + K))
	})

	ucbStats := NewSearchStats(MaxThreads)
	raveStats := NewSearchStats(MaxThreads)

	for i := range MaxThreads {
		threads := i + 1
		fmt.Printf("Running search with %d threads...\n", threads)

		ucbTree.SetLimits(mcts.DefaultLimits().SetCycles(uint32(100000 * threads)).SetThreads(threads))
		ucbTree.Search()
		res := ucbTree.SearchResult(bestChildPolicy)
		ucbStats.Set(i, int(res.Cps), res.Depth, len(res.Lines[0].Pv), ucbTree.CollisionFactor())
		fmt.Printf("UCB1: %s\n", res.String())

		// Discard current search tree
		ucbTree.Reset()

		// RAVE
		raveTree.SetLimits(mcts.DefaultLimits().SetCycles(uint32(100000 * threads)).SetThreads(threads))
		raveTree.Search()
		res = raveTree.SearchResult(bestChildPolicy)
		raveStats.Set(i, int(res.Cps), res.Depth, len(res.Lines[0].Pv), raveTree.CollisionFactor())
		fmt.Printf("RAVE: %s\n", res.String())
		raveTree.Reset()
	}

	// Compare the results
	Summary(MaxThreads, ucbStats, raveStats)
}
