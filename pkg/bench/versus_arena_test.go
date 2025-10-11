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

func (dp *DummyPos) Clone() *DummyPos {
	history := make([]Move, len(dp.history))
	copy(history, dp.history)
	return &DummyPos{history: history}
}

func (dp *DummyPos) MakeMove(m Move) {
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

func (dp *DummyPos) IsTerminated() bool {
	return len(dp.history) >= 8
}

// A dummy implementation of NodeStatsLike[S] for testing purposes.
// Always expands by adding 'branchFactor' children, and does random rollouts (0 == draw, 1 == win, 2 == loss)

type DummyOps struct {
	depth    int
	rand     *rand.Rand
	slowDown bool
	pos      *DummyPos
}

func (d *DummyOps) SetSlowdown(s bool) { d.slowDown = s }
func (d DummyOps) Reset()              {}
func (d *DummyOps) Traverse(m Move)    { d.pos.MakeMove(m) }
func (d *DummyOps) BackTraverse()      { d.pos.Undo() }

func (d *DummyOps) ExpandNode(parent *mcts.NodeBase[Move, *mcts.NodeStats]) uint32 {
	// Always add 'branchFactor' children
	parent.Children = make([]mcts.NodeBase[Move, *mcts.NodeStats], branchFactor)
	for i := range parent.Children {
		move := Move(i)
		d.pos.MakeMove(move)
		term := d.pos.IsTerminated()
		d.pos.Undo()

		parent.Children[i] = *mcts.NewBaseNode(parent, move, term, &mcts.NodeStats{})
	}
	return branchFactor
}

func (d DummyOps) Rollout() mcts.Result {
	if d.slowDown {
		time.Sleep(time.Duration((d.rand.Int()%100)+50) * time.Microsecond)
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

func (d DummyOps) Clone() *DummyOps {
	history := make([]Move, len(d.pos.history))
	copy(history, d.pos.history)
	return &DummyOps{depth: d.depth, pos: &DummyPos{history: history}, slowDown: d.slowDown}
}

// A dummy MCTS implementation for testing purposes.
type DummyMCTS struct {
	mcts.MCTS[Move, *mcts.NodeStats, mcts.Result, *DummyOps]
}

func NewDummyMCTS(policy mcts.MultithreadPolicy) *DummyMCTS {
	return &DummyMCTS{
		MCTS: *mcts.NewMTCS(
			mcts.UCB1, &DummyOps{pos: NewDummyPos(), slowDown: true}, policy,
			&mcts.NodeStats{},
			mcts.DefaultBackprop[Move, *mcts.NodeStats, mcts.Result, *DummyOps]{},
		),
	}
}

func (dmcts *DummyMCTS) Reset() {
	dmcts.MCTS.Reset(false, &mcts.NodeStats{})
}

func (dmcts *DummyMCTS) Search() Move {
	dmcts.MCTS.SearchMultiThreaded()
	dmcts.MCTS.Synchronize()
	return dmcts.MCTS.BestMove()
}

func (dmcts *DummyMCTS) SetPosition(p *DummyPos) {
	dmcts.Ops().pos = p
	dmcts.Reset()
}

func (dmcts *DummyMCTS) Clone() ExtMCTS[Move, *mcts.NodeStats, mcts.Result, *DummyPos] {
	newMCTS := NewDummyMCTS(mcts.MultithreadTreeParallel)
	newMCTS.Limiter.SetLimits(dmcts.Limiter.Limits())
	return newMCTS
}

func TestMain(m *testing.M) {
	mcts.SetSeedGeneratorFn(func() int64 {
		return time.Now().UnixNano()
	})
	fmt.Printf("Using seed %d\n", mcts.SeedGeneratorFn())

	os.Exit(m.Run())
}

func GetDummyMCTS() *DummyMCTS {
	tree := NewDummyMCTS(mcts.MultithreadTreeParallel)
	tree.Limiter.SetLimits(mcts.DefaultLimits().SetCycles(10000))
	tree.SearchMultiThreaded()
	tree.Synchronize()
	return tree
}

func TestBasicListeners(t *testing.T) {
	t1 := NewDummyMCTS(mcts.MultithreadTreeParallel)
	t2 := NewDummyMCTS(mcts.MultithreadTreeParallel)
	arena := NewVersusArena(NewDummyPos(), t1, t2)

	arena.Setup(mcts.DefaultLimits().SetCycles(1000), 5, 4)
	arena.Start(&DefaultListener[Move]{})

	arena.Wait()
}
