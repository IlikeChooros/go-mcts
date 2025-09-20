package uttt

/*

Ultimate Tic Tac Toe MCTS implementation




*/

import (
	"go-mcts/pkg/mcts"
	"math/rand"
	"time"
	"unsafe"
)

// Actual UTTT mcts implementation
type UtttMCTS struct {
	mcts.MCTS[PosType]
	ops *UtttOperations
}

type UtttNode mcts.NodeBase[PosType]

func NewUtttMCTS(position Position) *UtttMCTS {
	// Each mcts instance must have its own operations instance
	uttt_ops := newUtttOps(position)
	ops := mcts.GameOperations[PosType](uttt_ops)
	tree := &UtttMCTS{
		MCTS: *mcts.NewMTCS(
			mcts.UCB1,
			ops,
			mcts.TerminalFlag(position.IsTerminated()),
			mcts.MultithreadTreeParallel,
		),
		ops: uttt_ops,
	}
	return tree
}

func (mcts *UtttMCTS) AsyncSearch() {
	mcts.MCTS.SearchMultiThreaded(mcts.ops)
}

// Start the search
func (mcts *UtttMCTS) Search() {

	// Run the search
	mcts.SearchMultiThreaded(mcts.ops)

	// Wait for the search to end
	mcts.Synchronize()
}

// Default selection used for debugging
func (mcts *UtttMCTS) Selection() *mcts.NodeBase[PosType] {
	return mcts.MCTS.Selection(mcts.Root, mcts.ops, rand.New(rand.NewSource(time.Now().UnixNano())), 0)
}

// Default backprop used for debugging
func (mcts *UtttMCTS) Backpropagate(node *mcts.NodeBase[PosType], result mcts.Result) {
	mcts.MCTS.Backpropagate(mcts.ops, node, result)
}

func (mcts *UtttMCTS) Ops() mcts.GameOperations[PosType] {
	return mcts.ops
}

func (mcts *UtttMCTS) Reset() {
	mcts.MCTS.Reset(mcts.ops, mcts.ops.position.IsTerminated())
}

// Set the position
func (mcts *UtttMCTS) SetPosition(position Position) {
	mcts.ops.position = position
	mcts.Reset()
}

func (mcts *UtttMCTS) SetNotation(notation string) error {
	defer mcts.Reset()
	return mcts.ops.position.FromNotation(notation)
}

func (tree *UtttMCTS) SearchResult(pvPolicy mcts.BestChildPolicy) SearchResult {

	multipv := tree.MultiPv(pvPolicy)
	result := SearchResult{
		Cps:    tree.Cps(),
		Depth:  tree.MaxDepth(),
		Cycles: tree.Root.Visits(),
		Lines:  make([]EngineLine, len(multipv)),
		Turn:   tree.ops.rootSide,
		Size:   tree.Size(),
		Memory: uint64(unsafe.Sizeof(mcts.NodeBase[PosType]{})) * uint64(tree.Size()),
	}

	for i := range len(multipv) {
		pvResult := multipv[i]
		line := &result.Lines[i]
		line.Pv = pvResult.Pv

		// Set the score
		if pvResult.Terminal {
			if pvResult.Draw {
				line.ScoreType = ValueScore
				line.Value = 50
			} else {
				line.ScoreType = MateScore
				line.Value = len(pvResult.Pv)

				// If the game ends on our turn, we are losing
				if line.Value%2 == 0 {
					line.Value = -line.Value
				}
			}
		} else {
			line.ScoreType = ValueScore
			if pvResult.Root.Visits() == 0 {
				line.Value = 50
			} else {
				line.Value = int(100 * pvResult.Root.AvgOutcome())
			}
		}
	}
	return result
}

type UtttOperations struct {
	position Position
	rootSide TurnType
	random   *rand.Rand
}

func newUtttOps(pos Position) *UtttOperations {
	return &UtttOperations{
		position: pos,
		rootSide: pos.Turn(),
		random:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (ops *UtttOperations) Reset() {
	ops.rootSide = ops.position.Turn()
}

func (ops *UtttOperations) ExpandNode(node *mcts.NodeBase[PosType]) uint32 {

	moves := ops.position.GenerateMoves()
	node.Children = make([]mcts.NodeBase[PosType], moves.size)

	for i, m := range moves.Slice() {
		ops.position.MakeMove(m)
		isTerminal := ops.position.IsTerminated()
		ops.position.UndoMove()

		node.Children[i] = *mcts.NewBaseNode(node, m, isTerminal)
	}

	return uint32(moves.size)
}

func (ops *UtttOperations) Traverse(signature PosType) {
	ops.position.MakeMove(signature)
}

func (ops *UtttOperations) BackTraverse() {
	ops.position.UndoMove()
}

// Play the game until a terminal node is reached
func (ops *UtttOperations) Rollout() mcts.Result {
	var moves *MoveList
	var move PosType
	var result mcts.Result = 0.5
	var moveCount int = 0
	leafTurn := ops.position.Turn()

	for !ops.position.IsTerminated() {
		moveCount++
		moves = ops.position.GenerateMoves()

		// Choose at random move
		move = moves.moves[ops.random.Int31()%int32(moves.size)]
		ops.position.MakeMove(move)
	}

	// If that's not a draw
	if t := ops.position.termination; (t == TerminationCircleWon && leafTurn == CircleTurn) ||
		(t == TerminationCrossWon && leafTurn == CrossTurn) {
		result = 1.0
		// We lost
	} else if t != TerminationDraw {
		result = 0.0
	}

	// Undo the moves
	for range moveCount {
		ops.position.UndoMove()
	}

	return result
}

func (ops UtttOperations) Clone() mcts.GameOperations[PosType] {
	return mcts.GameOperations[PosType](&UtttOperations{
		position: ops.position.Clone(),
		rootSide: ops.rootSide,
		random:   rand.New(rand.NewSource(time.Now().UnixMicro())),
	})
}
