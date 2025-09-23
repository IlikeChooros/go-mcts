package mcts

import (
	"math"
)

// Rapid Action Value Estimation (RAVE) selection policy
// Reference: https://en.wikipedia.org/wiki/Monte_Carlo_tree_search#Improvements
// Use this only for game with highly branching factor and transposable states,
// meaning the moves can be played in different order from given position, and the result
// will be the same. For example: Go, Chess, Tic Tac Toe (transposable positions).

// Added playouts and outcomes contating node's move
type RaveStats struct {
	NodeStats

	// Float64 value with 10^-3 percision, stored as uint64
	OutcomesContatingMove int32

	// Number of nodes below this node's parent, containing this node's move
	PlayoutsContainingMove int32
}

// Source: https://en.wikipedia.org/wiki/Monte_Carlo_tree_search#Improvements
// function should be close to one and to zero for relatively small and relatively big 'playouts' and 'playoutsContatingMove' respectively.
type RaveBetaFnType func(playouts, playoutsContatingMove uint64) float64

func RaveDSilver(playouts, playoutsContatingMove uint64) float64 {
	const (
		b      = 0.5
		factor = 4 * b * b
	)
	return float64(playouts) / (float64(playouts+playoutsContatingMove) + factor*float64(playouts*playoutsContatingMove))
}

// Customizable beta function for the rave selection, by default uses D. Silver solution
var RaveBetaFunction RaveBetaFnType = RaveDSilver

// Default node selection policy (upper confidence bound)
func RAVE[T MoveLike, S NodeStatsLike](parent, root *NodeBase[T, S]) *NodeBase[T, S] {

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
