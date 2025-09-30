package chessmcts

/*
Chess MCTS (UCB1) example

This example shows how to plug the go-mcts library into a third-party chess engine
for move generation and legality testing. It uses:
  - Game state and moves from: github.com/IlikeChooros/dragontoothmg (fork of dylhunn's dragontooth,
  which is no longer maintained, or at least not updated for 3 years now)
  - Selection policy: UCB1 (Upper Confidence Bound)
  - Node statistics: mcts.NodeStats (win/visit counters + virtual loss)
  - Backpropagation: DefaultBackprop (2-player zero-sum)

Important integration points:
  - ExpandNode must enumerate legal moves from the current board state and append children nodes.
  - Traverse applies a move to the board; BackTraverse undoes it.
  - Rollout performs a light playout (random moves) until termination and returns a result in [0.0, 1.0].
  - SetRand is invoked by the MCTS worker to provide a per-thread RNG; use it for randomized rollouts.

Threading:
  - We run with MultithreadTreeParallel by default (shared tree, atomics). For best scaling, keep ExpandNode
    side-effect free except the provided node initialization, and keep Rollout working only on the local board.
*/

import (
	"math/rand"

	chess "github.com/IlikeChooros/dragontoothmg"
	mcts "github.com/IlikeChooros/go-mcts/pkg/mcts"
)

// UcbGameOps implements mcts.GameOperations for chess using the dragontoothmg board.
// It owns the board state and is cloned per worker for thread safety.
type UcbGameOps struct {
	board  *chess.Board
	random *rand.Rand // injected by the search worker via SetRand
}

// UcbMctsType wires the generic MCTS to the chess-specific operations and types.
type UcbMctsType struct {
	mcts.MCTS[chess.Move, *mcts.NodeStats, mcts.Result]
	ops *UcbGameOps
}

// NewUcbMcts constructs a ready-to-search MCTS instance for chess with UCB1 selection.
func NewUcbMcts() *UcbMctsType {
	ops := newUcbGameOps()
	Mcts := &UcbMctsType{
		MCTS: *mcts.NewMTCS(
			mcts.UCB1,                    // selection policy
			ops,                          // game operations (implements Expand/Traverse/Rollout/etc.)
			0,                            // initial flags (set TerminalFlag if your root is game-over)
			mcts.MultithreadTreeParallel, // threading policy
			&mcts.NodeStats{},            // default stats for new nodes
			mcts.DefaultBackprop[chess.Move, *mcts.NodeStats, mcts.Result]{}, // standard 2-player backprop
		),
		ops: ops,
	}
	return Mcts
}

// Search runs a (possibly multi-threaded) search and waits until done.
func (ucb *UcbMctsType) Search() {
	ucb.SearchMultiThreaded(ucb.ops)
	ucb.Synchronize()
}

// newUcbGameOps creates a fresh operations object starting from the initial chess position.
func newUcbGameOps() *UcbGameOps {
	return &UcbGameOps{
		board: chess.NewBoard(),
	}
}

// ExpandNode adds all legal child moves under p, initializing each child node.
// It must not mutate global/shared state; only the provided node and local board.
func (o *UcbGameOps) ExpandNode(p *mcts.NodeBase[chess.Move, *mcts.NodeStats]) uint32 {
	moves := o.board.GenerateLegalMoves()
	p.Children = make([]mcts.NodeBase[chess.Move, *mcts.NodeStats], len(moves))

	for i := range moves {
		// Make to test terminality of the resulting position, then undo.
		o.board.Make(moves[i])
		isTerminal := o.board.IsTerminated(len(o.board.GenerateLegalMoves()))
		o.board.Undo()

		// Initialize the child node with the move and terminal flag.
		p.Children[i] = *mcts.NewBaseNode(p, moves[i], isTerminal, &mcts.NodeStats{})
	}

	return uint32(len(moves))
}

// Traverse applies a move to the board when descending the tree.
func (o *UcbGameOps) Traverse(m chess.Move) {
	o.board.Make(m)
}

// BackTraverse undoes the last move when ascending during backpropagation.
func (o *UcbGameOps) BackTraverse() {
	o.board.Undo()
}

// Rollout plays random moves until the game terminates and returns the result
// from the perspective of the side-to-move at the rollout start as a float in [0,1].
// 1.0 = win, 0.0 = loss, 0.5 = draw.
func (o *UcbGameOps) Rollout() mcts.Result {
	var result mcts.Result = 0.5
	moveCount := 0
	leafIsWhite := o.board.Wtomove

	// Generate initial moves once; subsequent iterations update it after making a move.
	moves := o.board.GenerateLegalMoves()
	for !o.board.IsTerminated(len(moves)) {
		moveCount++

		// Choose a random legal move using the worker-provided RNG.
		o.board.Make(moves[o.random.Int()%len(moves)])
		moves = o.board.GenerateLegalMoves()
	}

	// If checkmate, assign win/loss relative to the leaf side to move.
	if o.board.Termination() == chess.TerminationCheckmate {
		if o.board.Wtomove == leafIsWhite {
			result = 0.0 // checkmated (opponent just moved); we lost
		} else {
			result = 1.0 // we delivered mate
		}
	}
	// Else stalemate or other draw, keep 0.5.

	// Rewind the board back to the leaf state.
	for range moveCount {
		o.board.Undo()
	}

	return result
}

// Reset allows you to clear any per-search state (none needed here).
func (o *UcbGameOps) Reset() {}

// SetRand is called once per worker to provide a thread-local RNG.
func (o *UcbGameOps) SetRand(r *rand.Rand) {
	o.random = r
}

// Clone returns a deep copy of the operations object for worker threads.
// The board must be cloned so each worker can mutate it independently.
func (o *UcbGameOps) Clone() mcts.GameOperations[chess.Move, *mcts.NodeStats, mcts.Result] {
	return &UcbGameOps{
		board: o.board.Clone(),
	}
}
