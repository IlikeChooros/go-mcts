package mcts

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
)

const (
	branchFactor = 20
)

type Move int

// A dummy implementation of NodeStatsLike for testing purposes.
// Always expands by adding 'branchFactor' children, and does random rollouts (0 == draw, 1 == win, 2 == loss)

type DummyOps struct {
	depth int
	rand  *rand.Rand
}

func (d DummyOps) Reset()           {}
func (d *DummyOps) Traverse(m Move) { d.depth++ }
func (d *DummyOps) BackTraverse()   { d.depth-- }

func (d *DummyOps) ExpandNode(parent *NodeBase[Move, *NodeStats]) uint32 {
	// Always add 'branchFactor' children
	parent.Children = make([]NodeBase[Move, *NodeStats], branchFactor)
	for i := range parent.Children {
		move := Move(i)
		parent.Children[i] = *NewBaseNode(parent, move, d.depth >= 8, &NodeStats{})
	}
	return branchFactor
}

func (d DummyOps) Rollout() Result {
	r := d.rand.Intn(3)
	switch r {
	case 0:
		return 0.5 // draw
	case 1:
		return 1.0 // win
	default:
		return 0.0 // loss
	}
}

func (d *DummyOps) SetRand(r *rand.Rand) {
	d.rand = r
}

func (d DummyOps) Clone() GameOperations[Move, *NodeStats, Result] {
	return &DummyOps{depth: d.depth}
}

// A dummy MCTS implementation for testing purposes.
type DummyMCTS struct {
	MCTS[Move, *NodeStats, Result]
	ops *DummyOps
}

func NewDummyMCTS(policy MultithreadPolicy) *DummyMCTS {
	ops := &DummyOps{}
	return &DummyMCTS{
		MCTS: *NewMTCS(
			UCB1, ops, 0, policy,
			&NodeStats{}, DefaultBackprop[Move, *NodeStats, Result]{},
		),
		ops: ops,
	}
}

func TestMain(m *testing.M) {
	SetSeedGeneratorFn(func() int64 {
		return 42
	})
	fmt.Printf("Using seed %d\n", SeedGeneratorFn())

	os.Exit(m.Run())
}

func GetDummyMCTS() *DummyMCTS {
	mcts := NewDummyMCTS(MultithreadTreeParallel)
	mcts.Limiter.SetLimits(DefaultLimits().SetCycles(10000))
	mcts.SearchMultiThreaded(mcts.ops)
	mcts.Synchronize()
	return mcts
}

// Tests checking if the search is working correctly

func TestDummySearch(t *testing.T) {
	mcts := GetDummyMCTS()

	if len(mcts.Root.Children) == 0 {
		t.Fatal("No children found after search")
	}

	pv, _, _ := mcts.Pv(mcts.Root, BestChildMostVisits, false)
	t.Logf("eval %.2f cps %d cycles %d pv %v", mcts.RootScore(), mcts.Cps(), mcts.Cycles(), pv)
}

func TestDummySearchWithListener(t *testing.T) {
	mcts := NewDummyMCTS(MultithreadTreeParallel)
	mcts.SetLimits(DefaultLimits().SetCycles(10000).SetThreads(4))
	listener := NewStatsListener[Move]()
	listener.
		OnDepth(func(stats ListenerTreeStats[Move]) {
			mainLine := stats.Lines[0]
			t.Logf("depth %d cycle %d cps %d eval %.2f pv %v", stats.Maxdepth, stats.Cycles, stats.Cps, mainLine.Eval, mainLine.Moves)
		}).
		OnCycle(func(stats ListenerTreeStats[Move]) {
			mainLine := stats.Lines[0]
			t.Logf("cycle %d depth %d cps %d eval %.2f pv %v", stats.Cycles, stats.Maxdepth, stats.Cps, mainLine.Eval, mainLine.Moves)
		}).
		SetCycleInterval(2000).
		OnStop(func(stats ListenerTreeStats[Move]) {
			mainLine := stats.Lines[0]
			t.Logf("stop reason %s after %d cycles, maxdepth %d cps %d pv %v", stats.StopReason, stats.Cycles, stats.Maxdepth, stats.Cps, mainLine.Moves)
		})

	mcts.SetListener(listener)
	mcts.SearchMultiThreaded(mcts.ops)
	mcts.Synchronize()

	// We expect some pv
	pv, _, _ := mcts.Pv(mcts.Root, BestChildMostVisits, false)
	if len(pv) <= 2 {
		t.Fatalf("No pv found after search, %v", pv)
	}
}

func TestDummySearchRootParallel(t *testing.T) {
	mcts := NewDummyMCTS(MultithreadRootParallel)
	mcts.Limiter.SetLimits(DefaultLimits().SetCycles(10000).SetThreads(4))
	mcts.SearchMultiThreaded(mcts.ops)
	mcts.Synchronize()

	if len(mcts.Root.Children) == 0 {
		t.Fatal("No children found after search")
	}

	pv, _, _ := mcts.Pv(mcts.Root, BestChildMostVisits, false)
	t.Logf("eval %.2f cps %d cycles %d pv %v", mcts.RootScore(), mcts.Cps(), mcts.Cycles(), pv)
}

// Actual unit tests for MCTS components, like Node cloning, UCB1 calculation, etc.

func TestMakeMove(t *testing.T) {
	tree := GetDummyMCTS()

	// Save the current stats
	maxdepth := tree.MaxDepth()
	size := tree.Size()
	pv, _, _ := tree.Pv(tree.Root, BestChildMostVisits, false)

	// Now make the best move, meaning the first move in the pv
	if len(pv) <= 2 {
		t.Fatalf("No pv found after search, %v", pv)
	}

	// Now check if the memory was released
	// treeMemBefore := tree.MemoryUsage()
	// memBefore := runtime.MemStats{}
	// runtime.ReadMemStats(&memBefore)
	// t.Logf("Memory before GC: Alloc %d TotalAlloc %d Sys %d NumGC %d", memBefore.Alloc, memBefore.TotalAlloc, memBefore.Sys, memBefore.NumGC)

	tree.MakeMove(pv[0])

	// runtime.GC()
	// memAfter := runtime.MemStats{}
	// runtime.ReadMemStats(&memAfter)
	// t.Logf("Memory after GC: Alloc %d TotalAlloc %d Sys %d NumGC %d", memAfter.Alloc, memAfter.TotalAlloc, memAfter.Sys, memAfter.NumGC)
	// t.Logf("Expected decrease %dB, actual %dB", treeMemBefore-tree.MemoryUsage(), memBefore.Alloc-memAfter.Alloc)

	// if memAfter.Alloc >= memBefore.Alloc {
	// 	t.Fatalf("Memory not released after MakeMove, before %d, after %d", memBefore.Alloc, memAfter.Alloc)
	// }
	if tree.MaxDepth() >= maxdepth {
		t.Fatalf("Max depth not decreased after MakeMove, was %d, now %d", maxdepth, tree.MaxDepth())
	}
	if tree.Size() >= size {
		t.Fatalf("Tree size not decreased after MakeMove, was %d, now %d", size, tree.Size())
	}

	newPv, _, _ := tree.Pv(tree.Root, BestChildMostVisits, false)
	if len(newPv) <= 1 {
		t.Fatalf("No pv found after MakeMove, %v", newPv)
	}

	if len(pv)-1 != len(newPv) {
		t.Fatalf("PV length not decreased after MakeMove, was %d, now %d", len(pv), len(newPv))
	}

	t.Logf("Tree size before move: %d, after move: %d", size, tree.Size())
	t.Logf("Pv before move: %v, after move: %v", pv, newPv)

	for i := range newPv {
		if pv[i+1] != newPv[i] {
			t.Fatalf("PV move %d not matching after MakeMove, was %v, now %v", i, pv, newPv)
		}
	}
}

func deepCompare(n1, n2 *NodeBase[Move, *NodeStats]) bool {
	if n1 == n2 {
		return true
	}
	if n1 == nil || n2 == nil {
		return false
	}
	if n1.Move != n2.Move {
		return false
	}
	if n1.Flags != n2.Flags {
		return false
	}
	if n1.Stats.N() != n2.Stats.N() || n1.Stats.RawQ() != n2.Stats.RawQ() {
		return false
	}
	if len(n1.Children) != len(n2.Children) {
		return false
	}
	for i := range n1.Children {
		if !deepCompare(&n1.Children[i], &n2.Children[i]) {
			return false
		}
	}
	return true
}

func TestNodeClone(t *testing.T) {
	// Create a sample tree
	tree := GetDummyMCTS()
	clone := tree.Root.Clone()

	if !deepCompare(tree.Root, clone) {
		t.Fatal("Cloned node does not match original")
	}

	// If this is true for the root, then it must be true for all children,
	// no need to check all of them
	for i := range tree.Root.Children {
		if clone.Children[i].Parent != clone {
			t.Fatal("Cloned child's parent does not point to cloned node")
		}
	}
}
