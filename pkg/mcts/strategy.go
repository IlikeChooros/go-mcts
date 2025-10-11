package mcts

type StrategyLike[T MoveLike, S NodeStatsLike[S], R GameResult, O GameOperations[T, S, R, O]] interface {
	Backpropagate(ops O, node *NodeBase[T, S], result R)
}

type DefaultBackprop[T MoveLike, S NodeStatsLike[S], R GameResult, O GameOperations[T, S, R, O]] struct{}

// Assumes the game is 2 player and zero sum, meaning for given result for the current player,
// the value for the enemy is exactly 1 - result
func (b DefaultBackprop[T, S, R, O]) Backpropagate(ops O, node *NodeBase[T, S], result Result) {
	/*
		source: https://en.wikipedia.org/wiki/Monte_Carlo_tree_search
			If white loses the simulation, all nodes along the selection incremented their simulation count (the denominator),
			but among them only the black nodes were credited with wins (the numerator). If instead white wins,
			all nodes along the selection would still increment their simulation count, but among them
			only the white nodes would be credited with wins. In games where draws are possible,
			a draw causes the numerator for both black and white to be incremented by 0.5 and the denominator by 1.
			This ensures that during selection, each player's choices expand towards the most promising moves for that player,
			which mirrors the goal of each player to maximize the value of their move.
	*/

	for node != nil {

		// Reverse virtual loss for non-root
		if node.Parent != nil {
			node.Stats.AddVvl(1-VirtualLoss, -VirtualLoss)
		} else {
			node.Stats.AddVvl(1, 0)
		}

		result = 1.0 - result // switch the result
		// Add the outcome
		node.Stats.AddQ(result)

		// Backpropagate
		node = node.Parent
		ops.BackTraverse()
	}
}
