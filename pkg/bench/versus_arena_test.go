package bench

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/IlikeChooros/go-mcts/pkg/mcts"
)

const (
	branchFactor = 20
)

type Move int

type DummyPos struct {
	history []Move
}

func NewDummyPos() *DummyPos {
	return &DummyPos{history: make([]Move, 0, 10)}
}

func (dp *DummyPos) Make(m Move) {
	dp.history = append(dp.history, m)
}

func (dp *DummyPos) Undo() {
	if len(dp.history) != 0 {
		dp.history = dp.history[:len(dp.history)-1]
	}
}

func (dp *DummyPos) Reset() {
	dp.history = make([]Move, 0, 10)
}

func (dp *DummyPos) IsDraw() bool {
	if len(dp.history) >= 8 && dp.history[len(dp.history)-1]%3 == 0 {
		return true
	}
	return false
}

func (dp *DummyPos) IsTerminal() bool {
	return len(dp.history) >= 8
}

// A dummy implementation of NodeStatsLike for testing purposes.
// Always expands by adding 'branchFactor' children, and does random rollouts (0 == draw, 1 == win, 2 == loss)

type DummyOps struct {
	depth    int
	rand     *rand.Rand
	slowDown bool
	pos      *DummyPos
}

func (d *DummyOps) SetSlowdown(s bool) { d.slowDown = s }
func (d DummyOps) Reset()              {}
func (d *DummyOps) Traverse(m Move)    { d.pos.Make(m) }
func (d *DummyOps) BackTraverse()      { d.pos.Undo() }

func (d *DummyOps) ExpandNode(parent *mcts.NodeBase[Move, *mcts.NodeStats]) uint32 {
	// Always add 'branchFactor' children
	parent.Children = make([]mcts.NodeBase[Move, *mcts.NodeStats], branchFactor)
	for i := range parent.Children {
		move := Move(i)
		d.pos.Make(move)
		term := d.pos.IsTerminal()
		d.pos.Undo()

		parent.Children[i] = *mcts.NewBaseNode(parent, move, term, &mcts.NodeStats{})
	}
	return branchFactor
}

func (d DummyOps) Rollout() mcts.Result {
	if d.slowDown {
		time.Sleep(10 * time.Millisecond)
	}
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

func (d DummyOps) Clone() mcts.GameOperations[Move, *mcts.NodeStats, mcts.Result] {
	return &DummyOps{depth: d.depth}
}

// A dummy MCTS implementation for testing purposes.
type DummyMCTS struct {
	mcts.MCTS[Move, *mcts.NodeStats, mcts.Result]
	ops *DummyOps
}

func NewDummyMCTS(policy mcts.MultithreadPolicy) *DummyMCTS {
	ops := &DummyOps{pos: NewDummyPos()}
	return &DummyMCTS{
		MCTS: *mcts.NewMTCS(
			mcts.UCB1, ops, 0, policy,
			&mcts.NodeStats{}, mcts.DefaultBackprop[Move, *mcts.NodeStats, mcts.Result]{},
		),
		ops: ops,
	}
}

func (dmcts *DummyMCTS) Reset() {
	dmcts.MCTS.Reset(dmcts.ops, false, &mcts.NodeStats{})
}

func (dmcts *DummyMCTS) Search() Move {
	dmcts.MCTS.SearchMultiThreaded(dmcts.ops)
	dmcts.MCTS.Synchronize()
	return dmcts.MCTS.BestMove()
}

func (dmcts *DummyMCTS) SetPosition(p PositionLike[Move]) {
	dmcts.ops.pos = p.(*DummyPos)
	dmcts.Reset()
}

func (dmcts *DummyMCTS) Clone() ExtMCTS[Move, *mcts.NodeStats, mcts.Result] {
	newMCTS := NewDummyMCTS(mcts.MultithreadTreeParallel)
	newMCTS.Limiter.SetLimits(dmcts.Limiter.Limits())
	return newMCTS
}

func TestMain(m *testing.M) {
	mcts.SetSeedGeneratorFn(func() int64 {
		return 42
	})
	fmt.Printf("Using seed %d\n", mcts.SeedGeneratorFn())

	os.Exit(m.Run())
}

func GetDummyMCTS() *DummyMCTS {
	tree := NewDummyMCTS(mcts.MultithreadTreeParallel)
	tree.Limiter.SetLimits(mcts.DefaultLimits().SetCycles(10000))
	tree.SearchMultiThreaded(tree.ops)
	tree.Synchronize()
	return tree
}

func TestBasicListeners(t *testing.T) {
	t1 := NewDummyMCTS(mcts.MultithreadTreeParallel)
	t2 := NewDummyMCTS(mcts.MultithreadTreeParallel)
	arena := NewVersusArena(NewDummyPos(), t1, t2)

	arena.Setup(mcts.DefaultLimits().SetCycles(1000), 4, 4)
	arena.Start(&DefaultListener[Move]{})

	arena.Wait()
}
