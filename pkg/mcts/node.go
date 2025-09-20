package mcts

import (
	"fmt"
	"sync/atomic"
)

// visits/virutal loss/win/loss count of the node,
// wins, and losses should be accessed only with atomic operations
// However to read the visit and virtual loss counts, use the methods
type NodeStats struct {
	sumOutcomes atomic.Uint64 // float64 value of compounded outcomes for this node with 10^-3 precision

	// This is visit counter, it cannot be read by atomic, use GetVvl() Visits() to properly read this value
	visits atomic.Int32

	// Current virtual loss applied to visits, it always meets condition: visits - virtualLoss >= 0.
	// Read this value ONLY with GetVvl() or VirtualLoss() methods
	virtualLoss atomic.Int32
}

const (
	CanExpand     uint32 = 0
	ExpandingMask uint32 = 1
	ExpandedMask  uint32 = 2
	TerminalMask  uint32 = 4
)

type NodeBase[T MoveLike] struct {
	NodeStats
	NodeSignature T
	Children      []NodeBase[T]
	Parent        *NodeBase[T]
	Flags         uint32 // must be read/written atomically
}

func newRootNode[T MoveLike](terminated bool) *NodeBase[T] {
	return &NodeBase[T]{
		Children: nil,
		Flags:    TerminalFlag(terminated),
	}
}

func NewBaseNode[T MoveLike](parent *NodeBase[T], signature T, terminated bool) *NodeBase[T] {
	return &NodeBase[T]{
		NodeSignature: signature,
		Children:      nil,
		Parent:        parent,
		Flags:         TerminalFlag(terminated), // flip the turn
	}
}

func (node *NodeBase[T]) Clone() *NodeBase[T] {
	clone := &NodeBase[T]{
		NodeSignature: node.NodeSignature,
		Children:      make([]NodeBase[T], len(node.Children)),
		Parent:        node.Parent,
		Flags:         node.Flags,
	}
	// Create a deep copy of the children
	for i := range node.Children {
		childClone := node.Children[i].Clone()
		clone.Children[i].sumOutcomes.Store(childClone.sumOutcomes.Load())
		clone.Children[i].visits.Store(childClone.visits.Load())
		clone.Children[i].virtualLoss.Store(childClone.virtualLoss.Load())
		clone.Children[i].NodeSignature = childClone.NodeSignature
		clone.Children[i].Parent = clone
		clone.Children[i].Flags = childClone.Flags
	}
	clone.visits.Store(node.visits.Load())
	clone.virtualLoss.Store(node.virtualLoss.Load())
	clone.sumOutcomes.Store(node.sumOutcomes.Load())
	return clone
}

func (node *NodeBase[T]) AvgOutcome() Result {
	return Result(node.sumOutcomes.Load()) / 1e3 / Result(node.Visits())
}

func (node *NodeBase[T]) Outcomes() Result {
	return Result(node.sumOutcomes.Load()) / 1e3
}

func (node *NodeBase[T]) AddOutcome(result Result) {
	node.sumOutcomes.Add(uint64(result * 1e3))
}

func (node *NodeBase[T]) Visits() int32 {
	return node.visits.Load()
}

func (node *NodeBase[T]) VirtualLoss() int32 {
	return node.virtualLoss.Load()
}

// Get both visits and virtual loss (to avoid situtation one of them is modified)
// returns (visits, virtual loss)
func (node *NodeBase[T]) GetVvl() (int32, int32) {
	// cas loop, so we can read the values atomically
	for {
		visits := node.visits.Load()
		virtualLoss := node.virtualLoss.Load()

		// Always preserve the condition that actual visits >= 0
		if virtualLoss <= visits {
			return visits, virtualLoss
		}
	}
}

// Returns visits - virtual loss
func (node *NodeBase[T]) RealVisits() int32 {
	visits, virtualLoss := node.GetVvl()
	return visits - virtualLoss
}

// Adds VirtuaLoss to both visits and virtual loss counters
func (node *NodeBase[T]) AddVvl(visits, virtualLoss int32) {
	node.virtualLoss.Add(virtualLoss)
	node.visits.Add(visits)
}

// Sets visits and virtual loss of this node to specified value
func (node *NodeBase[T]) SetVvl(visits, virtualLoss int32) {
	node.virtualLoss.Store(virtualLoss)
	node.visits.Store(visits)

	// If the virtual loss is greater than visits, we have a problem
	if virtualLoss > visits {
		panic(fmt.Sprintf("Virtual loss (%d) cannot be greater than visits (%d)", virtualLoss, visits))
	}
}

// Reads the game Flags, and return wheter the node is terminal
func (node *NodeBase[T]) Terminal() bool {
	return atomic.LoadUint32(&node.Flags)&TerminalMask == TerminalMask
}

func (node *NodeBase[T]) SetFlag(flag uint32) {
	atomic.StoreUint32(&node.Flags, flag)
}

func TerminalFlag(terminal bool) uint32 {
	flag := uint32(0)
	if terminal {
		flag |= TerminalMask
	}
	return flag
}

// Same as asking if the node has chidlren
func (node *NodeBase[T]) Expanded() bool {
	return atomic.LoadUint32(&node.Flags)&ExpandedMask == ExpandedMask
}

// See if currenlty node is being expanded
func (node *NodeBase[T]) Expanding() bool {
	return atomic.LoadUint32(&node.Flags)&ExpandingMask == ExpandingMask
}

// Should be called when we want to expand this node,
// if it's possible, sets the internal flag to 'currently expanding'
func (node *NodeBase[T]) CanExpand() bool {
	// TODO:
	// This is causing the concurrent threads to deadlock in 'Expanding' loop
	return atomic.CompareAndSwapUint32(&node.Flags, CanExpand, ExpandingMask)
}

// After successful 'CanExpand' call, use this function to set
// the state of the node to 'expanded'
func (node *NodeBase[T]) FinishExpanding() {
	atomic.StoreUint32(&node.Flags, ExpandedMask)
}
