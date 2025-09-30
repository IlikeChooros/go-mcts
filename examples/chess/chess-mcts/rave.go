package chessmcts

/*
Chess MCTS (RAVE/AMAF) example

This example is similar to the UCB version but uses RAVE (Rapid Action Value Estimation)
to blend standard Q-values with all-moves-as-first (AMAF) statistics. This tends to help
in high-branching, transposable domains like chess.

Key differences vs UCB example:
  - Node stats type is mcts.RaveStats (implements RaveStatsLike).
  - Backpropagation strategy is mcts.RaveBackprop[…], which updates AMAF counters.
  - Rollout returns a RaveGameResult that carries:
      * The playout result in [0,1]
      * The sequence of moves played by white and by black, so ancestors can update AMAF.

Integration notes:
  - ExpandNode is unchanged in spirit: generate legal moves, init child nodes.
  - Rollout must track moves by side-to-move so AMAF updates can match ancestor turns.
*/

import (
	"math/rand"

	chess "github.com/IlikeChooros/dragontoothmg"
	mcts "github.com/IlikeChooros/go-mcts/pkg/mcts"
)

// RaveGameOps implements the chess-specific GameOperations for RAVE mode.
type RaveGameOps struct {
	board  *chess.Board
	random *rand.Rand // injected by the worker
}

// RaveGameResult implements mcts.RaveGameResult[chess.Move].
// It stores the playout result and the moves played, split by side, and tracks
// whose turn it is to align move lists with the tree path during backprop.
type RaveGameResult struct {
	wmoves  []chess.Move // moves played by white during the playout
	bmoves  []chess.Move // moves played by black during the playout
	wtomove bool         // current player at the moment (used during backprop switch)
	result  mcts.Result  // [0,1] from the perspective of the rollout root
}

// Value returns the playout result in [0,1].
func (r *RaveGameResult) Value() mcts.Result {
	return r.result
}

// Moves returns the list of moves played by the current player.
// Backprop will call SwitchTurn to alternate which list is active.
func (r *RaveGameResult) Moves() []chess.Move {
	mvs := r.bmoves
	if r.wtomove {
		mvs = r.wmoves
	}
	return mvs
}

// Append pushes a move into the current player's move list during backprop.
func (r *RaveGameResult) Append(m chess.Move) {
	mvptr := &r.bmoves
	if r.wtomove {
		mvptr = &r.wmoves
	}
	*mvptr = append(*mvptr, m)
}

// SwitchTurn flips the side-to-move context so Append/Moves target the other list.
func (r *RaveGameResult) SwitchTurn() {
	r.wtomove = !r.wtomove
}

// RaveMctsType wires go-mcts with chess types and the RAVE policy and stats.
type RaveMctsType struct {
	mcts.MCTS[chess.Move, *mcts.RaveStats, *RaveGameResult]
	ops *RaveGameOps
}

// NewRaveMcts constructs a ready-to-search MCTS instance configured for RAVE.
func NewRaveMcts() *RaveMctsType {
	ops := newRaveGameOps()
	Mcts := &RaveMctsType{
		MCTS: *mcts.NewMTCS(
			mcts.RAVE, // RAVE selection policy
			ops,
			0,
			mcts.MultithreadTreeParallel,
			&mcts.RaveStats{}, // stats that include AMAF counters
			mcts.RaveBackprop[chess.Move, *mcts.RaveStats, *RaveGameResult]{}, // backprop with AMAF updates
		),
		ops: ops,
	}
	return Mcts
}

// Search runs the search and waits for completion.
func (ucb *RaveMctsType) Search() {
	ucb.SearchMultiThreaded(ucb.ops)
	ucb.Synchronize()
}

// newRaveGameOps creates a fresh operations object at the initial position.
func newRaveGameOps() *RaveGameOps {
	return &RaveGameOps{
		board: chess.NewBoard(),
	}
}

// ExpandNode adds all legal child moves under p, initializing each to RaveStats.
func (o *RaveGameOps) ExpandNode(p *mcts.NodeBase[chess.Move, *mcts.RaveStats]) uint32 {
	moves := o.board.GenerateLegalMoves()
	p.Children = make([]mcts.NodeBase[chess.Move, *mcts.RaveStats], len(moves))

	for i := range moves {
		o.board.Make(moves[i])
		isTerminal := o.board.IsTerminated(len(o.board.GenerateLegalMoves()))
		o.board.Undo()

		p.Children[i] = *mcts.NewBaseNode(p, moves[i], isTerminal, &mcts.RaveStats{})
	}

	return uint32(len(moves))
}

// Traverse applies a move to the board when descending the tree.
func (o *RaveGameOps) Traverse(m chess.Move) {
	o.board.Make(m)
}

// BackTraverse undoes the last move when ascending during backpropagation.
func (o *RaveGameOps) BackTraverse() {
	o.board.Undo()
}

// Rollout plays random moves until termination and returns both the
// numerical result and the move lists needed for RAVE/AMAF updates.
//
// Implementation details:
//   - We record moves in wmoves/bmoves based on side-to-move at each step.
//   - At the end, we translate termination into [0,1] from the leaf's perspective.
//   - Finally, we rewind the board back to the leaf state.
func (o *RaveGameOps) Rollout() *RaveGameResult {
	var result mcts.Result = 0.5
	moveCount := 0
	leafIsWhite := o.board.Wtomove
	wmoves := make([]chess.Move, 0, 128)
	bmoves := make([]chess.Move, 0, 128)

	moves := o.board.GenerateLegalMoves()
	for !o.board.IsTerminated(len(moves)) {
		moveCount++

		m := moves[o.random.Int()%len(moves)]
		if o.board.Wtomove {
			wmoves = append(wmoves, m)
		} else {
			bmoves = append(bmoves, m)
		}

		o.board.Make(m)
		moves = o.board.GenerateLegalMoves()
	}

	// Assign result for checkmate, else keep draw at 0.5.
	if o.board.Termination() == chess.TerminationCheckmate {
		if o.board.Wtomove == leafIsWhite {
			result = 0.0
		} else {
			result = 1.0
		}
	}

	// Rewind the board back to the leaf state.
	for range moveCount {
		o.board.Undo()
	}

	return &RaveGameResult{
		result:  result,
		wtomove: leafIsWhite,
		wmoves:  wmoves,
		bmoves:  bmoves,
	}
}

// Reset allows resetting per-search state (none needed here).
func (o *RaveGameOps) Reset() {}

// SetRand is called once per worker to inject a thread-local RNG.
func (o *RaveGameOps) SetRand(r *rand.Rand) {
	o.random = r
}

// Clone returns a deep copy of the operations object for worker threads.
// Each worker receives its own board instance to mutate independently.
func (o *RaveGameOps) Clone() mcts.GameOperations[chess.Move, *mcts.RaveStats, *RaveGameResult] {
	return &RaveGameOps{
		board: o.board.Clone(),
	}
}
