package rave_uttt

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	uttt "github.com/IlikeChooros/go-mcts/examples/ultimate-tic-tac-toe/uttt/core"
	mcts "github.com/IlikeChooros/go-mcts/pkg/mcts"
)

// Default selection used for debugging
func (tree *UtttMCTS) Selection() *mcts.NodeBase[uttt.PosType, *mcts.RaveStats] {
	return tree.MCTS.Selection(tree.Root, tree.ops, rand.New(rand.NewSource(time.Now().UnixNano())), 0)
}

// Default backprop used for debugging
func (tree *UtttMCTS) Backpropagate(node *mcts.NodeBase[uttt.PosType, *mcts.RaveStats], result *UtttGameResult) {
	tree.MCTS.Strategy().Backpropagate(tree.ops, node, result)
}

func (tree *UtttMCTS) Ops() mcts.RaveGameOperations[uttt.PosType, *mcts.RaveStats, *UtttGameResult] {
	return tree.ops
}

func TestMCTSRollout(t *testing.T) {
	positions := []string{
		uttt.StartingPosition,
		"1x7/2o6/x8/9/9/9/9/9/9 o -",
		"9/9/9/7x1/4xo3/8x/9/4o4/o8 x -",
	}

	for _, notation := range positions {
		t.Run(fmt.Sprintf("Rollout-%s", strings.ReplaceAll(notation, "/", "|")), func(t *testing.T) {
			pos := uttt.NewPosition()
			err := pos.FromNotation(notation)
			if err != nil {
				t.Fatal(err)
			}

			ops := &UtttOperations{position: *pos, random: rand.New(rand.NewSource(22))}
			originalNotation := pos.Notation()

			// Perform rollout
			result := ops.Rollout()

			// Check result is valid
			if v := result.Value(); v < 0 || v > 1 {
				t.Errorf("Invalid rollout result value: %f", v)
			}

			// There should be moves
			if mvs := result.Moves(); len(mvs) == 0 {
				t.Errorf("Rollout moves are empty")
			}

			// Position should be restored
			if pos.Notation() != originalNotation {
				t.Error("Position not restored after rollout")
			}
		})
	}
}

func TestMCTSSelection(t *testing.T) {
	pos := uttt.NewPosition()
	err := pos.FromNotation(uttt.StartingPosition)
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
	pos := uttt.NewPosition()
	err := pos.FromNotation(uttt.StartingPosition)
	if err != nil {
		t.Fatal(err)
	}

	tree := NewUtttMCTS(*pos)

	// Create a simple tree structure
	tree.Root.Stats.SetVvl(1, 0)

	child := &tree.Root.Children[0]
	child.Stats.SetVvl(int32(mcts.VirtualLoss), int32(mcts.VirtualLoss))

	// Test backpropagation with win
	originalNotation := pos.Notation()
	tree.Backpropagate(child, &UtttGameResult{
		turn:   pos.Turn(),
		result: 0.0,
	})

	// Check statistics
	if child.Stats.Visits() != 1 {
		t.Errorf("Child visits should be 1, got %d", child.Stats.Visits())
	}
	if int(child.Stats.Outcomes()) != 1 {
		t.Errorf("Child wins should be 1, got %f", child.Stats.Outcomes())
	}
	if tree.Root.Stats.Visits() != 2 { // Original 1 + 1 from backprop
		t.Errorf("Root visits should be 2, got %d", tree.Root.Stats.Visits())
	}
	// Position should be restored
	if pos.Notation() != originalNotation {
		t.Error("Position not restored after backpropagation")
	}
}

func TestMCTSSearch(t *testing.T) {
	pos := uttt.NewPosition()
	err := pos.FromNotation(uttt.StartingPosition)
	if err != nil {
		t.Fatal(err)
	}

	mcts := NewUtttMCTS(*pos)

	// Set short time limit for testing
	mcts.Limits().SetCycles(10000)
	originalNotation := pos.Notation()

	// Run search
	mcts.Search()

	// Check that search actually ran
	if mcts.Root.Stats.Visits() == 0 {
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
