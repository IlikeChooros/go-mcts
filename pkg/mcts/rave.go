package mcts

import (
	"math"
	"slices"
	"sync/atomic"
)

// Rapid Action Value Estimation (RAVE) selection policy
// Reference: https://en.wikipedia.org/wiki/Monte_Carlo_tree_search#Improvements
// Use this only for game with highly branching factor and transposable states,
// meaning the moves can be played in different order from given position, and the result
// will be the same. For example: Go, Chess, Tic Tac Toe (transposable positions).

type RaveStatsLike[S any] interface {
	NodeStatsLike[S]

	// Outcomes contating node's move
	QRAVE() Result
	RawQRAVE() int32
	// Playouts contating node's move
	NRAVE() int32
	// Add new outcome, that contains node's move
	AddQRAVE(Result)
	// Increment playouts with
	AddNRAVE(int32)
}

// Added playouts and outcomes contating node's move, holds AMAF statistics
type RaveStats struct {
	NodeStats

	// Float64 value with 10^-3 percision, stored as uint64
	q_rave int32

	// Number of nodes below this node's parent, containing this node's move
	n_rave int32
}

func DefaultRaveStats() *RaveStats {
	return &RaveStats{}
}

func (r *RaveStats) Clone() *RaveStats {
	return &RaveStats{
		NodeStats: NodeStats{
			q:           atomic.LoadUint64(&r.q),
			n:           atomic.LoadInt32(&r.n),
			virtualLoss: atomic.LoadInt32(&r.virtualLoss),
		},
		q_rave: r.RawQRAVE(),
		n_rave: r.NRAVE(),
	}
}

func (r *RaveStats) QRAVE() Result {
	return Result(atomic.LoadInt32(&r.q_rave)) / Result(1e3)
}

func (r *RaveStats) RawQRAVE() int32 {
	return atomic.LoadInt32(&r.q_rave)
}

func (r *RaveStats) NRAVE() int32 {
	return atomic.LoadInt32(&r.n_rave)
}

func (r *RaveStats) AddQRAVE(result Result) {
	atomic.AddInt32(&r.q_rave, int32(result*1e3))
}

func (r *RaveStats) AddNRAVE(playouts int32) {
	atomic.AddInt32(&r.n_rave, playouts)
}

// Source: https://en.wikipedia.org/wiki/Monte_Carlo_tree_search#Improvements
// function should be close to one and to zero for relatively small and relatively big 'n' and 'n_rave' respectively.
type RaveBetaFnType func(n, n_rave int32) float64

func RaveDSilver(n, n_rave int32) float64 {
	const (
		b      = 0.1
		factor = 4 * b * b
	)
	return float64(n) / (float64(n+n_rave) + factor*float64(n*n_rave))
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

type RaveGameOperations[T MoveLike, S RaveStatsLike[S], R RaveGameResult[T], O GameOperations[T, S, R, O]] interface {
	GameOperations[T, S, R, O]
}

// Rapid Action Value Estimation (RAVE)
// Reference: https://en.wikipedia.org/wiki/Monte_Carlo_tree_search#Improvements
type RAVE[T MoveLike, S RaveStatsLike[S], R RaveGameResult[T], O GameOperations[T, S, R, O]] struct {
	ExplorationParam float64
	BetaFunction     RaveBetaFnType
}

func NewRAVE[T MoveLike, S RaveStatsLike[S], R RaveGameResult[T], O GameOperations[T, S, R, O]]() *RAVE[T, S, R, O] {
	return &RAVE[T, S, R, O]{
		ExplorationParam: 0.3, // lower exploration, because of AMAF
		BetaFunction:     RaveDSilver,
	}
}

func (r *RAVE[T, S, R, O]) SetExplorationParam(c float64) *RAVE[T, S, R, O] {
	r.ExplorationParam = c
	return r
}

// Source: https://en.wikipedia.org/wiki/Monte_Carlo_tree_search#Improvements
// function should be close to one and to zero for relatively small and relatively big 'n' and 'n_rave' respectively.
func (r *RAVE[T, S, R, O]) SetBetaFunction(f RaveBetaFnType) *RAVE[T, S, R, O] {
	r.BetaFunction = f
	return r
}

func (r RAVE[T, S, R, O]) Select(parent, root *NodeBase[T, S]) *NodeBase[T, S] {
	// Is that's a terminal node, simply return itself, there is no children anyway
	// and on the rollout we will exit early, since the position is terminated
	if parent.Terminal() {
		return parent
	}

	var child *NodeBase[T, S]
	var actualVisits, visits, vl int32

	max := float64(-1)
	index := 0
	lnParentVisits := math.Log(float64(parent.Stats.N()))

	for i := 0; i < len(parent.Children); i++ {

		// Get the variables
		child = &parent.Children[i]
		visits, vl = child.Stats.GetVvl()
		actualVisits = visits - vl

		// Pick the unvisited one
		if actualVisits == 0 {
			return child
		}

		q := float64(child.Stats.Q()) / float64(visits)
		b := 0.0
		amafq := 0.0
		if nRave := child.Stats.NRAVE(); nRave > 0 {
			// specified in vars.go
			b = r.BetaFunction(actualVisits, nRave)
			amafq = float64(child.Stats.QRAVE()) / float64(nRave)
		}

		ucb := (1.0-b)*q + b*amafq +
			r.ExplorationParam*math.Sqrt(lnParentVisits/float64(visits))

		if ucb > max {
			max = ucb
			index = i
		}
	}

	return &parent.Children[index]
}

func (b RAVE[T, S, R, O]) Backpropagate(ops O, node *NodeBase[T, S], result R) {
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
		node.Stats.AddQ(v)

		// Reverse virtual loss for non-root
		if node.Parent != nil {
			node.Stats.AddVvl(1-VirtualLoss, -VirtualLoss)

			mvs := result.Moves()
			var ch *NodeBase[T, S]
			for i := range node.Parent.Children {
				// Check if the child contains a move from the playout
				ch = &node.Parent.Children[i]
				if slices.Contains(mvs, ch.Move) {
					ch.Stats.AddQRAVE(v)
					ch.Stats.AddNRAVE(1)
				}
			}

			result.Append(node.Move)
		} else {
			node.Stats.AddVvl(1, 0)
		}

		// Backpropagate
		result.SwitchTurn()
		node = node.Parent
		ops.BackTraverse()
	}
}
