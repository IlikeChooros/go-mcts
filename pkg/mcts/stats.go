package mcts

import (
	"fmt"
	"sync/atomic"
)

type NodeStatsLike interface {
	N() int32
	VirtualLoss() int32
	AddQ(Result)
	AvgQ() Result
	Q() Result
	RawQ() uint64
	SetVvl(visits, vl int32)
	GetVvl() (visits int32, vl int32)
	AddVvl(visits, vl int32)
	RealVisits() int32
	Clone() NodeStatsLike
}

// visits/virutal draw/win/loss count of the node,
// However to read the visit and virtual loss counts, use the methods
type NodeStats struct {
	q uint64 // float64 value of compounded outcomes for this node with 10^-3 precision

	// This is visit counter, it cannot be read by atomic, use GetVvl() N() to properly read this value
	n int32

	// Current virtual loss applied to visits, it always meets condition: visits - virtualLoss >= 0.
	// Read this value ONLY with GetVvl() or VirtualLoss() methods
	virtualLoss int32
}

func (stats *NodeStats) Clone() NodeStatsLike {
	return &NodeStats{
		q:           atomic.LoadUint64(&stats.q),
		n:           atomic.LoadInt32(&stats.n),
		virtualLoss: atomic.LoadInt32(&stats.virtualLoss),
	}
}

// Average outcome for this node
func (stats *NodeStats) AvgQ() Result {
	return Result((atomic.LoadUint64(&stats.q))) / 1e3 / Result(stats.N())
}

// Cumulated rewards/outcomes for this node
func (stats *NodeStats) Q() Result {
	return Result(atomic.LoadUint64(&stats.q)) / 1e3
}

// Raw cumulated rewards/outcomes for this node, with 10^-3 precision
func (stats *NodeStats) RawQ() uint64 {
	return atomic.LoadUint64(&stats.q)
}

// Add new outcome to this node
func (stats *NodeStats) AddQ(result Result) {
	atomic.AddUint64(&stats.q, uint64(result*1e3))
}

// Get number of visits to this node
func (stats *NodeStats) N() int32 {
	return atomic.LoadInt32(&stats.n)
}

func (stats *NodeStats) VirtualLoss() int32 {
	return atomic.LoadInt32(&stats.virtualLoss)
}

// Get both visits and virtual loss (to avoid situtation one of them is modified)
// returns (visits, virtual loss)
func (stats *NodeStats) GetVvl() (visits int32, virtualLoss int32) {
	// cas loop, so we can read the values atomically
	for {
		visits = atomic.LoadInt32(&stats.n)
		virtualLoss = atomic.LoadInt32(&stats.virtualLoss)

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
	atomic.AddInt32(&stats.n, visits)
}

// Sets visits and virtual loss of this stats to specified value
func (stats *NodeStats) SetVvl(visits, virtualLoss int32) {
	atomic.StoreInt32(&stats.virtualLoss, virtualLoss)
	atomic.StoreInt32(&stats.n, visits)

	// If the virtual loss is greater than visits, we have a problem
	if virtualLoss > visits {
		panic(fmt.Sprintf("Virtual loss (%d) cannot be greater than visits (%d)", virtualLoss, visits))
	}
}
