package mcts

import "time"

// Main thread id, which has some privileges, like calling the listener during the search
const mainThreadId = 0

// Virtual loss value, used in multithreaded MCTS[T, S, R, O, A]to avoid multiple threads
// exploring the same node simultaneously
const VirtualLoss int32 = 2

var SeedGeneratorFn SeedGeneratorFnType = func() int64 {
	return time.Now().UnixNano()
}

// Set custom seed generator function for random number generators in MCTS[T, S, R, O, A]
// by default uses current time in nanoseconds
func SetSeedGeneratorFn(f SeedGeneratorFnType) {
	if f != nil {
		SeedGeneratorFn = f
	}
}

const (
	// Parallel building of the same game tree, protecting data from simultaneous writes
	// using atomic operations. Best approach for most cases.
	MultithreadTreeParallel MultithreadPolicy = iota

	// This will spawn multiple threads (specified by Limits.NThreads) that will
	// build independent game trees in parallel. Note that the listener will be called
	// only on the main thread, so the evaluation and pv will be inaccurate until
	// the results are merged after the search is done.
	MultithreadRootParallel
)

const (
	// When choosing the best child, choose the one with most visits,
	// this is the go-to method for MCTS
	BestChildMostVisits BestChildPolicy = iota

	// Experimental: choose the child with the best win rate
	BestChildWinRate
)
