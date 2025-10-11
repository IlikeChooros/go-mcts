package mcts

import "math"

type UCB1[T MoveLike, S NodeStatsLike[S], R GameResult, O GameOperations[T, S, R, O]] struct {
	ExplorationParam float64
}

func (u *UCB1[T, S, R, O]) SetExplorationParam(c float64) {
	u.ExplorationParam = max(0, c)
}

func NewUCB1[T MoveLike, S NodeStatsLike[S], R GameResult, O GameOperations[T, S, R, O]](explorationParam float64) *UCB1[T, S, R, O] {
	return &UCB1[T, S, R, O]{ExplorationParam: explorationParam}
}

func (u *UCB1[T, S, R, O]) Select(parent, root *NodeBase[T, S]) *NodeBase[T, S] {

	if parent.Terminal() {
		return parent
	}

	max := float64(-1)
	index := 0
	lnParentVisits := math.Log(float64(parent.Stats.N()))
	var child *NodeBase[T, S]
	var actualVisits, visits, vl int32
	var wins Result

	for i := 0; i < len(parent.Children); i++ {

		// Get the variables
		child = &parent.Children[i]
		visits, vl = child.Stats.GetVvl()
		actualVisits = visits - vl

		// Pick the unvisited one
		if actualVisits == 0 {
			// Return pointer to the child
			return child
		}

		wins = child.Stats.Q()

		// UCB 1 : wins/visits + C * sqrt(ln(parent_visits)/visits)
		// ucb1 = epliotation + exploration
		// Since we assume the game is zero-sum, we want to expand the tree's nodes
		// that have best value according to the root
		ucb1 := float64(wins)/float64(visits) +
			u.ExplorationParam*math.Sqrt(lnParentVisits/float64(visits))

		if ucb1 > max {
			max = ucb1
			index = i
		}
	}

	return &parent.Children[index]
}

func (b *UCB1[T, S, R, O]) Backpropagate(ops O, node *NodeBase[T, S], result Result) {
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
