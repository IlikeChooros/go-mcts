package mcts

import "time"

// Main thread id, which has some privileges, like calling the listener during the search
const mainThreadId = 0

// Virtual loss value, used in multithreaded MCTS to avoid multiple threads
// exploring the same node simultaneously
const VirtualLoss int32 = 2

// Exploration parameter used in UCB1 formula, higher values increase exploration
// while lower values increase exploitation. Theoretical perfect value is sqrt(2), but it has to be tuned for each problem.
// Default is 0.75
var ExplorationParam float64 = 0.75

// Set the exploration parameter used in UCB1 formula
func SetExplorationParam(c float64) {
	ExplorationParam = max(0.0, c)
}

// Customizable beta function for the rave selection, by default uses D. Silver solution, with b=0.1
var RaveBetaFunction RaveBetaFnType = RaveDSilver

// Set custom beta function for RAVE selection policy
func SetRaveBetaFunction(f RaveBetaFnType) {
	if f != nil {
		RaveBetaFunction = f
	}
}

var SeedGeneratorFn SeedGeneratorFnType = func() int64 {
	return time.Now().UnixNano()
}

// Set custom seed generator function for random number generators in MCTS,
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
