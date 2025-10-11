package mcts

// Other types, which didn't fit to MCTS[T, S, R, O, A]or Node files

// Result of the rollout, should range from [0, 1] - 0 being loss from the leaf's node perspective
// and 1 being a win
type Result float64
type MoveLike comparable
type BestChildPolicy int
type MultithreadPolicy int
type SeedGeneratorFnType func() int64
