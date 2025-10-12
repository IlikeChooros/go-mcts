package mcts

import (
	"sync/atomic"
)

const (
	CanExpand     uint32 = 0
	ExpandingMask uint32 = 1
	ExpandedMask  uint32 = 2
	TerminalMask  uint32 = 4
)

type NodeBase[T MoveLike, S NodeStatsLike[S]] struct {
	Stats    S
	Move     T
	Children []NodeBase[T, S]
	Parent   *NodeBase[T, S]
	Flags    uint32 // must be read/written atomically
}

type NodeBaseDefault[T MoveLike] NodeBase[T, *NodeStats]

func newRootNode[T MoveLike, S NodeStatsLike[S]](terminated bool, defaultStats S) *NodeBase[T, S] {
	return &NodeBase[T, S]{
		Children: nil,
		Flags:    TerminalFlag(terminated),
		Stats:    defaultStats,
	}
}

// NewBaseNode creates a new child node under the given parent with the specified move.
func NewBaseNode[T MoveLike, S NodeStatsLike[S]](parent *NodeBase[T, S], move T, terminated bool, defaultStats S) *NodeBase[T, S] {
	return &NodeBase[T, S]{
		Move:     move,
		Children: nil,
		Parent:   parent,
		Stats:    defaultStats,
		Flags:    TerminalFlag(terminated),
	}
}

func (node *NodeBase[T, S]) Clone(parent *NodeBase[T, S]) *NodeBase[T, S] {
	// TODO:
	// Test this properly

	clone := &NodeBase[T, S]{
		Move:     node.Move,
		Children: make([]NodeBase[T, S], len(node.Children)),
		Parent:   parent,
		Flags:    node.Flags,
	}
	// Create a deep copy of the children
	for i := range node.Children {
		// childClone := node.Children[i].Clone()
		// clone.Children[i].Stats = childClone.Stats.Clone().(S)
		// clone.Children[i].Move = childClone.Move
		// clone.Children[i].Parent = clone
		// clone.Children[i].Flags = childClone.Flags
		clone.Children[i] = *node.Children[i].Clone(clone)
	}
	// clone.visits.Store(node.visits.Load())
	// clone.virtualLoss.Store(node.virtualLoss.Load())
	// clone.sumOutcomes.Store(node.sumOutcomes.Load())
	clone.Stats = node.Stats.Clone()
	return clone
}

// Reads the game Flags, and return wheter the stats is terminal
func (stats *NodeBase[T, S]) Terminal() bool {
	return atomic.LoadUint32(&stats.Flags)&TerminalMask == TerminalMask
}

func (stats *NodeBase[T, S]) SetFlag(flag uint32) {
	atomic.StoreUint32(&stats.Flags, flag)
}

func TerminalFlag(terminal bool) uint32 {
	flag := uint32(0)
	if terminal {
		flag |= TerminalMask
	}
	return flag
}

// Same as asking if the node has chidlren
func (node *NodeBase[T, S]) Expanded() bool {
	return atomic.LoadUint32(&node.Flags)&ExpandedMask == ExpandedMask
}

// See if currenlty node is being expanded
func (node *NodeBase[T, S]) Expanding() bool {
	return atomic.LoadUint32(&node.Flags)&ExpandingMask == ExpandingMask
}

// Should be called when we want to expand this node,
// if it's possible, sets the internal flag to 'currently expanding'
func (node *NodeBase[T, S]) CanExpand() bool {
	// TODO:
	// This is causing the concurrent threads to deadlock in 'Expanding' loop
	return atomic.CompareAndSwapUint32(&node.Flags, CanExpand, ExpandingMask)
}

// Used to undo the expanding state, if something went wrong
// during the expansion (failed allocation of children)
func (node *NodeBase[T, S]) CancelExpanding() {
	atomic.StoreUint32(&node.Flags, CanExpand)
}

// After successful 'CanExpand' call, use this function to set
// the state of the node to 'expanded'
func (node *NodeBase[T, S]) FinishExpanding() {
	atomic.StoreUint32(&node.Flags, ExpandedMask)
}
