package mcts

// Other types, which didn't fit to MCTS or Node files

// Result of the rollout, should range from [0, 1] - 0 being loss from the leaf's node perspective
// and 1 being a win
type Result float64
type MoveLike comparable
type BestChildPolicy int
type MultithreadPolicy int

// Will be called, when we choose this node, as it is the most promising to expand
// Warning: when using NodeStats fields, must use atomic operations (Load, Store)
// since the search may be multi-threaded (tree parallelized)
type SelectionPolicy[T MoveLike, S NodeStatsLike[S]] func(parent, root *NodeBase[T, S]) *NodeBase[T, S]
type SeedGeneratorFnType func() int64
