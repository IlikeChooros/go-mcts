package mcts

import "math"

var ExplorationParam float64 = 0.75

// Set the exploration parameter used in UCB1 formula
func SetExplorationParam(c float64) {
	ExplorationParam = max(0.0, c)
}

// Default node selection policy (upper confidence bound)
func UCB1[T MoveLike, S NodeStatsLike](parent, root *NodeBase[T, S]) *NodeBase[T, S] {

	// Is that's a terminal node, simply return itself, there is no children anyway
	// and on the rollout we will exit early, since the position is terminated
	if parent.Terminal() {
		return parent
	}

	max := float64(-1)
	index := 0
	lnParentVisits := math.Log(float64(parent.Stats.Visits()))
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

		wins = child.Stats.Outcomes()

		// UCB 1 : wins/visits + C * sqrt(ln(parent_visits)/visits)
		// ucb1 = epliotation + exploration
		// Since we assume the game is zero-sum, we want to expand the tree's nodes
		// that have best value according to the root
		ucb1 := float64(wins)/float64(visits) +
			ExplorationParam*math.Sqrt(lnParentVisits/float64(visits))

		if ucb1 > max {
			max = ucb1
			index = i
		}
	}

	return &parent.Children[index]
}
