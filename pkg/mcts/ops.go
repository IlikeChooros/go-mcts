package mcts

import "math/rand"

type GameResult any

type GameOperations[T MoveLike, S NodeStatsLike[S], R GameResult, O any] interface {
	// Generate moves here, and add them as children to given node
	ExpandNode(parent *NodeBase[T, S]) uint32
	// Make a move on the internal position definition, with given
	// signature value (move)
	Traverse(T)
	// Go back up 1 time in the game tree (undo previous move, which was played in traverse)
	BackTraverse()
	// Function to make the playout, until terminal node is reached,
	// in case of UTTT, play random moves, until we reach draw/win/loss
	Rollout() R
	// Reset game state to current internal position, called after changing
	// position, for example using SetNotation function in engine
	Reset()
	// Clone itself, without any shared memory with the other object
	Clone() O
}

// Random-based rollout
type RandGameOperations[T MoveLike, S NodeStatsLike[S], R GameResult, O any] interface {
	GameOperations[T, S, R, O]
	// Sets the random genertor
	SetRand(*rand.Rand)
}
