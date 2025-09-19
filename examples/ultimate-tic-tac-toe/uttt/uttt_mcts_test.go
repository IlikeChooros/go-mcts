package uttt

import (
	"fmt"
	"go-mcts/pkg/mcts"
	"math"
	"math/rand"
	"strings"
	"testing"
)

func TestMCTSBasicFunctionality(t *testing.T) {
	pos := NewPosition()
	err := pos.FromNotation(StartingPosition)
	if err != nil {
		t.Fatal(err)
	}

	mcts := NewUtttMCTS(*pos)

	// Test initial state
	if mcts.Root == nil {
		t.Error("Root node should not be nil")
	}

	if mcts.Root.Visits() != 0 {
		t.Errorf("Initial visits should be 0, got %d", mcts.Root.Visits())
	}

	if mcts.Root.Children == nil {
		t.Error("Initial children shouldn't be nil")
	}
}

func TestMCTSExpansion(t *testing.T) {
	pos := NewPosition()
	err := pos.FromNotation(StartingPosition)
	if err != nil {
		t.Fatal(err)
	}

	ops := &UtttOperations{position: *pos}

	// Create a root node
	root := &mcts.NodeBase[PosType]{
		// Non-zero visits to trigger expansion
		Flags: mcts.TerminalFlag(false),
	}
	root.SetVvl(0, 0)

	// Test expansion
	ops.ExpandNode(root)

	if root.Children == nil {
		t.Error("Children should not be nil after expansion")
	}

	expectedMoves := pos.GenerateMoves().Size()
	if len(root.Children) != expectedMoves {
		t.Errorf("Expected %d children, got %d", expectedMoves, len(root.Children))
	}

	// Check parent pointers
	for i := range root.Children {
		child := &root.Children[i]
		if child.Parent != root {
			t.Errorf("Child %d parent pointer incorrect", i)
		}
	}
}

func TestMCTSTraverseBackTraverse(t *testing.T) {
	pos := NewPosition()
	err := pos.FromNotation(StartingPosition)
	if err != nil {
		t.Fatal(err)
	}

	ops := &UtttOperations{position: *pos}
	originalNotation := pos.Notation()

	// Get a valid move
	moves := pos.GenerateMoves()
	if moves.Size() == 0 {
		t.Fatal("No valid moves available")
	}

	move := moves.moves[0]

	// Test traverse
	ops.Traverse(move)
	if pos.Notation() == originalNotation {
		t.Error("Position should have changed after traverse")
	}

	// Test back traverse
	ops.BackTraverse()
	if pos.Notation() != originalNotation {
		t.Error("Position should be restored after back traverse")
	}
}

func TestMCTSRollout(t *testing.T) {
	positions := []string{
		StartingPosition,
		"1x7/2o6/x8/9/9/9/9/9/9 o -",
		"9/9/9/7x1/4xo3/8x/9/4o4/o8 x -",
	}

	for _, notation := range positions {
		t.Run(fmt.Sprintf("Rollout-%s", strings.ReplaceAll(notation, "/", "|")), func(t *testing.T) {
			pos := NewPosition()
			err := pos.FromNotation(notation)
			if err != nil {
				t.Fatal(err)
			}

			ops := &UtttOperations{position: *pos, random: rand.New(rand.NewSource(22))}
			originalNotation := pos.Notation()

			// Perform rollout
			result := ops.Rollout()

			// Check result is valid
			if result < 0 || result > 1 {
				t.Errorf("Invalid rollout result: %f", result)
			}

			// Position should be restored
			if pos.Notation() != originalNotation {
				t.Error("Position not restored after rollout")
			}
		})
	}
}

func TestMCTSSelection(t *testing.T) {
	pos := NewPosition()
	err := pos.FromNotation(StartingPosition)
	if err != nil {
		t.Fatal(err)
	}

	mcts := NewUtttMCTS(*pos)

	// Test selection with unvisited nodes
	selected := mcts.Selection()
	if selected == nil {
		t.Error("Selection should return a node")
	}

	// Position should be at the selected node
	expectedNotation := pos.Notation()
	pos.UndoMove() // Should undo the traverse from selection
	if pos.Notation() == expectedNotation {
		t.Error("Selection should have traversed to a different position")
	}
}

func TestMCTSBackpropagation(t *testing.T) {
	pos := NewPosition()
	err := pos.FromNotation(StartingPosition)
	if err != nil {
		t.Fatal(err)
	}

	tree := NewUtttMCTS(*pos)

	// Create a simple tree structure
	tree.Root.SetVvl(1, 0)

	child := &tree.Root.Children[0]
	child.SetVvl(int32(mcts.VirtualLoss), int32(mcts.VirtualLoss))

	// Test backpropagation with win
	originalNotation := pos.Notation()
	tree.Backpropagate(child, 0)

	// Check statistics
	if child.Visits() != 1 {
		t.Errorf("Child visits should be 1, got %d", child.Visits())
	}
	if int(child.Outcomes()) != 1 {
		t.Errorf("Child wins should be 1, got %f", child.Outcomes())
	}
	if tree.Root.Visits() != 2 { // Original 1 + 1 from backprop
		t.Errorf("Root visits should be 2, got %d", tree.Root.Visits())
	}
	// Position should be restored
	if pos.Notation() != originalNotation {
		t.Error("Position not restored after backpropagation")
	}
}

func TestMCTSSearch(t *testing.T) {
	pos := NewPosition()
	err := pos.FromNotation(StartingPosition)
	if err != nil {
		t.Fatal(err)
	}

	mcts := NewUtttMCTS(*pos)

	// Set short time limit for testing
	mcts.Limits().SetMovetime(100)
	originalNotation := pos.Notation()

	// Run search
	mcts.Search()

	// Check that search actually ran
	if mcts.Root.Visits() == 0 {
		t.Error("Root should have been visited during search")
	}

	// Position should be restored
	if pos.Notation() != originalNotation {
		t.Error("Position not restored after search")
	}

	// Should have children after search
	if mcts.Root.Children == nil {
		t.Error("Root should have children after search")
	}
}

func TestMCTSBestChild(t *testing.T) {
	pos := NewPosition()
	err := pos.FromNotation(StartingPosition)
	if err != nil {
		t.Fatal(err)
	}

	mcts := NewUtttMCTS(*pos)

	// Run a short search
	mcts.Limits().SetMovetime(50)
	mcts.Search()

	// Get best child
	bestMove := mcts.RootSignature()

	// Verify it's a legal move
	legalMoves := pos.GenerateMoves()
	found := false
	for _, move := range legalMoves.Slice() {
		if move == bestMove {
			found = true
			break
		}
	}

	if !found {
		t.Error("Best child should be a legal move")
	}
}

func TestMCTSUCB1Calculation(t *testing.T) {
	// Create mock nodes to test UCB1
	parent := &mcts.NodeBase[PosType]{
		NodeStats: mcts.NodeStats{},
	}
	parent.SetVvl(100, 0)

	children := []mcts.NodeBase[PosType]{
		{NodeStats: mcts.NodeStats{}, Parent: parent},
		{NodeStats: mcts.NodeStats{}, Parent: parent},
		{NodeStats: mcts.NodeStats{}, Parent: parent}, // Unvisited
	}

	children[0].SetVvl(10, 0)
	children[0].AddOutcome(7.0)
	children[1].SetVvl(8, 0)
	children[1].AddOutcome(3.0)
	children[2].SetVvl(0, 0)

	parent.Children = children

	// Test selection policy
	selected := mcts.UCB1(parent, parent)

	// Should select unvisited node
	if selected.Visits() != 0 {
		t.Error("Should select unvisited node first, picked:", selected)
	}

	// Remove unvisited node and test UCB1
	parent.Children = children[:2]
	selected = mcts.UCB1(parent, parent)

	// Verify UCB1 calculation makes sense
	if selected == nil {
		t.Error("Should select a node")
	}

	// Both nodes should have reasonable UCB1 values
	for i := range parent.Children {
		node := &parent.Children[i]
		if node.Visits() > 0 {
			winRate := float64(node.Outcomes()) / float64(node.Visits())
			exploration := mcts.ExplorationParam * math.Sqrt(math.Log(float64(parent.Visits()))/float64(node.Visits()))
			ucb1 := winRate + exploration

			if math.IsNaN(ucb1) || math.IsInf(ucb1, 0) {
				t.Errorf("UCB1 calculation resulted in NaN or Inf for node %d", i)
			}
		}
	}
}

func TestMCTSTerminalNodes(t *testing.T) {
	// Test near-terminal position
	pos := NewPosition()
	err := pos.FromNotation("xxx6/xxx6/xxx6/9/9/9/9/9/9 o -")
	if err != nil {
		t.Fatal(err)
	}

	mcts := NewUtttMCTS(*pos)

	// Should recognize terminal state
	if !mcts.Root.Terminal() {
		t.Error("Root should be terminal for finished game")
	}

	// Search should handle terminal nodes gracefully
	mcts.Limits().SetMovetime(50)
	mcts.Search()
}

func TestMCTSMultiThreadedSearch(t *testing.T) {
	// Test if multi threaded search returns proper search result

	pos, err := FromNotation(StartingPosition)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewUtttMCTS(*pos)
	engine.Limits().SetThreads(4).SetCycles(40000)
	engine.Search()

	result, _ := engine.SearchResult(mcts.BestChildWinRate).MainLine()

	if len(result.Pv) == 0 {
		t.Error("Pv shouldn't be empty after search")
	}
	if result.Bestmove == PosIllegal {
		t.Error("Bestmove is empty")
	}
	if result.Value == -1 {
		t.Error("Value should be correctly set")
	}

	t.Log(result)
}

func BenchmarkMCTSRollout(b *testing.B) {
	pos := NewPosition()
	err := pos.FromNotation(StartingPosition)
	if err != nil {
		b.Fatal(err)
	}

	ops := newUtttOps(*pos)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ops.Rollout()
	}
}
