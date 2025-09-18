package mcts

import (
	"encoding/json"
	"math"
	"strings"
)

type Limits struct {
	Depth    int
	Nodes    uint32
	Cycles   uint32
	Movetime int
	Infinite bool
	NThreads int
	ByteSize int64
	MultiPv  int
}

func (l Limits) String() string {
	builder := strings.Builder{}
	_ = json.NewEncoder(&builder).Encode(l)
	return builder.String()
}

const (
	DefaultDepthLimit    int    = math.MaxInt
	DefaultNodeLimit     uint32 = math.MaxInt32*2 + 1
	DefaultMovetimeLimit int    = -1
	DefaultByteSizeLimit int64  = -1
	DefaultCyclesLimit   uint32 = math.MaxInt32*2 + 1
)

func DefaultLimits() *Limits {
	return &Limits{
		Depth:    DefaultDepthLimit,
		Nodes:    DefaultNodeLimit,
		Cycles:   DefaultCyclesLimit,
		Movetime: DefaultMovetimeLimit,
		Infinite: true,
		NThreads: 1,
		ByteSize: DefaultByteSizeLimit,
		MultiPv:  1,
	}
}

// Set the maximum depth of the search
func (l *Limits) SetDepth(depth int) *Limits {
	l.Depth = depth
	l.Infinite = false
	return l
}

// Set the maxiumum number of nodes engine can go through
func (l *Limits) SetNodes(nodes uint32) *Limits {
	l.Nodes = nodes
	l.Infinite = false
	return l
}

// Set the number of backpropagation cycles in monte-carlo tree search
func (l *Limits) SetCycles(visits uint32) *Limits {
	l.Cycles = visits
	l.Infinite = false
	return l
}

// Set the maximum time for engine to think
func (l *Limits) SetMovetime(movetime int) *Limits {
	l.Movetime = movetime
	l.Infinite = false
	return l
}

func (l *Limits) SetInfinite(infinite bool) {
	l.Infinite = infinite
}

func (l *Limits) SetThreads(threads int) *Limits {
	l.NThreads = max(threads, 1)
	return l
}

func (l *Limits) SetMultiPv(multipv int) *Limits {
	l.MultiPv = max(1, multipv)
	return l
}

func (l *Limits) SetMbSize(mbsize int) *Limits {
	return l.SetByteSize(int64(mbsize) * (1 << 20))
}

func (l *Limits) SetByteSize(bytesize int64) *Limits {
	l.ByteSize = bytesize
	l.Infinite = false
	return l
}

func (l *Limits) InfiniteSize() bool {
	return l.ByteSize == -1
}
