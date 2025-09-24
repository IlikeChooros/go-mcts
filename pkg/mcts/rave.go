package mcts

import (
	"math"
	"sync/atomic"
)

// Rapid Action Value Estimation (RAVE) selection policy
// Reference: https://en.wikipedia.org/wiki/Monte_Carlo_tree_search#Improvements
// Use this only for game with highly branching factor and transposable states,
// meaning the moves can be played in different order from given position, and the result
// will be the same. For example: Go, Chess, Tic Tac Toe (transposable positions).

type RaveStatsLike interface {
	NodeStatsLike

	// Outcomes contating node's move
	RaveOCM() Result
	// # Playouts contating node's move
	RavePCM() int32
	// Add new outcome, that contains node's move
	AddRaveOCM(Result)
	// Increment playouts with
	AddRavePCM(int32)
}

// Added playouts and outcomes contating node's move
type RaveStats struct {
	NodeStats

	// Float64 value with 10^-3 percision, stored as uint64
	outcomesContainingMove int32

	// Number of nodes below this node's parent, containing this node's move
	playoutsContainingMove int32
}

func (r *RaveStats) RaveOCM() Result {
	return Result(atomic.LoadInt32(&r.outcomesContainingMove) / 1e3)
}

func (r *RaveStats) RavePCM() int32 {
	return atomic.LoadInt32(&r.playoutsContainingMove)
}

func (r *RaveStats) AddRaveOCM(result Result) {
	atomic.AddInt32(&r.outcomesContainingMove, int32(result*1e3))
}

func (r *RaveStats) AddRavePCM(playouts int32) {
	atomic.AddInt32(&r.playoutsContainingMove, playouts)
}

// Source: https://en.wikipedia.org/wiki/Monte_Carlo_tree_search#Improvements
// function should be close to one and to zero for relatively small and relatively big 'playouts' and 'playoutsContatingMove' respectively.
type RaveBetaFnType func(playouts, playoutsContatingMove int32) float64

func RaveDSilver(playouts, playoutsContatingMove int32) float64 {
	const (
		b      = 0.5
		factor = 4 * b * b
	)
	return float64(playouts) / (float64(playouts+playoutsContatingMove) + factor*float64(playouts*playoutsContatingMove))
}

// Customizable beta function for the rave selection, by default uses D. Silver solution
var RaveBetaFunction RaveBetaFnType = RaveDSilver

// Rapid Action Value Estimation (RAVE) selection policy
// Reference: https://en.wikipedia.org/wiki/Monte_Carlo_tree_search#Improvements
func RAVE[T MoveLike, S RaveStatsLike](parent, root *NodeBase[T, S]) *NodeBase[T, S] {

	// Is that's a terminal node, simply return itself, there is no children anyway
	// and on the rollout we will exit early, since the position is terminated
	if parent.Terminal() {
		return parent
	}

	var child *NodeBase[T, S]
	var actualVisits, visits, vl int32

	max := float64(-1)
	index := 0
	lnParentVisits := math.Log(float64(parent.Stats.Visits()))

	for i := 0; i < len(parent.Children); i++ {

		// Get the variables
		child = &parent.Children[i]
		visits, vl = child.Stats.GetVvl()
		actualVisits = visits - vl

		// Pick the unvisited one
		if actualVisits == 0 {
			return child
		}

		q := float64(child.Stats.Outcomes()) / float64(visits)
		b := 0.0
		rf := 0.0
		if pcm := child.Stats.RavePCM(); pcm > 0 {
			b = RaveBetaFunction(actualVisits, pcm)
			rf = float64(child.Stats.RaveOCM()) / float64(pcm)
		}

		ucb := (1.0-b)*q + b*rf +
			ExplorationParam*math.Sqrt(lnParentVisits/float64(visits))

		if ucb > max {
			max = ucb
			index = i
		}
	}

	return &parent.Children[index]
}

type RaveGameResult[T MoveLike] interface {
	// Result of the game
	Value() Result
	// Moves played in rollout, but only the ones played by current player
	Moves() []T
	// Append new move
	Append(T)
	// Switch turn
	SwitchTurn()
}

type RaveDefaultGameResult[T MoveLike] struct {
	v   Result
	mvs []T
}

func NewRaveGameResult[T MoveLike](v Result, mvs []T) *RaveDefaultGameResult[T] {
	return &RaveDefaultGameResult[T]{
		v: v, mvs: mvs,
	}
}

func (r *RaveDefaultGameResult[T]) Value() Result {
	return r.v
}

func (r *RaveDefaultGameResult[T]) Moves() []T {
	return r.mvs
}

func (r *RaveDefaultGameResult[T]) Append(move T) {

}

func (r *RaveDefaultGameResult[T]) SwitchTurn() {

}

type RaveGameOperations[T MoveLike, S RaveStatsLike, R RaveGameResult[T]] interface {
	GameOperations[T, S, R]
}

type RaveBackprop[T MoveLike, S RaveStatsLike, R RaveGameResult[T]] struct{}

func (b RaveBackprop[T, S, R]) Backpropagate(ops GameOperations[T, S, R], node *NodeBase[T, S], result R) {
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

	v := result.Value()

	for node != nil {

		v = 1.0 - v // switch the result
		// Add the outcome
		node.Stats.AddOutcome(v)

		// Reverse virtual loss for non-root
		if node.Parent != nil {
			node.Stats.AddVvl(1-VirtualLoss, -VirtualLoss)

			mvs := result.Moves()
			var ch *NodeBase[T, S]
			for i := range node.Parent.Children {
				// Check if the child contains a move from the playout
				ch = &node.Parent.Children[i]
				for j := range mvs {
					if ch.NodeSignature == mvs[j] {
						ch.Stats.AddRaveOCM(v)
						ch.Stats.AddRavePCM(1)
					}
				}
			}
		} else {
			node.Stats.AddVvl(1, 0)
		}

		// Backpropagate
		result.SwitchTurn()
		node = node.Parent
		ops.BackTraverse()
	}
}
