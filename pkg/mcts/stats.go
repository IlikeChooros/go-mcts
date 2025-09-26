package mcts

import (
	"fmt"
	"sync/atomic"
)

type NodeStatsLike interface {
	Visits() int32
	VirtualLoss() int32
	AddOutcome(Result)
	AvgOutcome() Result
	Outcomes() Result
	RawOutcomes() uint64
	SetVvl(visits, vl int32)
	GetVvl() (visits int32, vl int32)
	AddVvl(visits, vl int32)
	RealVisits() int32
	Clone() NodeStatsLike
}

// visits/virutal loss/win/loss count of the node,
// wins, and losses should be accessed only with atomic operations
// However to read the visit and virtual loss counts, use the methods
type NodeStats struct {
	sumOutcomes uint64 // float64 value of compounded outcomes for this node with 10^-3 precision

	// This is visit counter, it cannot be read by atomic, use GetVvl() Visits() to properly read this value
	visits int32

	// Current virtual loss applied to visits, it always meets condition: visits - virtualLoss >= 0.
	// Read this value ONLY with GetVvl() or VirtualLoss() methods
	virtualLoss int32
}

func (stats *NodeStats) Clone() NodeStatsLike {
	return &NodeStats{
		sumOutcomes: atomic.LoadUint64(&stats.sumOutcomes),
		visits:      atomic.LoadInt32(&stats.visits),
		virtualLoss: atomic.LoadInt32(&stats.virtualLoss),
	}
}

func (stats *NodeStats) AvgOutcome() Result {
	return Result(atomic.LoadUint64(&stats.sumOutcomes)) / 1e3 / Result(stats.Visits())
}

func (stats *NodeStats) Outcomes() Result {
	return Result(atomic.LoadUint64(&stats.sumOutcomes)) / 1e3
}

func (stats *NodeStats) RawOutcomes() uint64 {
	return atomic.LoadUint64(&stats.sumOutcomes)
}

func (stats *NodeStats) AddOutcome(result Result) {
	atomic.AddUint64(&stats.sumOutcomes, uint64(result*1e3))
}

func (stats *NodeStats) Visits() int32 {
	return atomic.LoadInt32(&stats.visits)
}

func (stats *NodeStats) VirtualLoss() int32 {
	return atomic.LoadInt32(&stats.virtualLoss)
}

// Get both visits and virtual loss (to avoid situtation one of them is modified)
// returns (visits, virtual loss)
func (stats *NodeStats) GetVvl() (int32, int32) {
	// cas loop, so we can read the values atomically
	for {
		visits := atomic.LoadInt32(&stats.visits)
		virtualLoss := atomic.LoadInt32(&stats.virtualLoss)

		// Always preserve the condition that actual visits >= 0
		if virtualLoss <= visits {
			return visits, virtualLoss
		}
	}
}

// Returns visits - virtual loss
func (stats *NodeStats) RealVisits() int32 {
	visits, virtualLoss := stats.GetVvl()
	return visits - virtualLoss
}

// Adds VirtuaLoss to both visits and virtual loss counters
func (stats *NodeStats) AddVvl(visits, virtualLoss int32) {
	atomic.AddInt32(&stats.virtualLoss, virtualLoss)
	atomic.AddInt32(&stats.visits, visits)
}

// Sets visits and virtual loss of this stats to specified value
func (stats *NodeStats) SetVvl(visits, virtualLoss int32) {
	atomic.StoreInt32(&stats.virtualLoss, virtualLoss)
	atomic.StoreInt32(&stats.visits, visits)

	// If the virtual loss is greater than visits, we have a problem
	if virtualLoss > visits {
		panic(fmt.Sprintf("Virtual loss (%d) cannot be greater than visits (%d)", virtualLoss, visits))
	}
}
