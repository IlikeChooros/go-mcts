package mcts

import (
	"math/rand"
	"runtime"
	"time"
)

// Use when started multi-threaded search and want it to synchronize with this thread
func (mcts *MCTS[T, S, R]) Synchronize() {
	if mcts.shouldMerge() {
		// Wait for the merge to finish
		for !mcts.merged.Load() {
			runtime.Gosched()
		}
	} else {
		// Just wait for all threads to finish
		mcts.wg.Wait()
	}
}

func (mcts *MCTS[T, S, R]) mergeResults() {
	for _, other := range mcts.roots[1:] {
		mergeResult(mcts.Root, other)
	}
	mcts.merged.Store(true)
	mcts.roots = nil
}

// Helper function to merge results from other root nodes into the main root
func mergeResult[T MoveLike, S NodeStatsLike](root *NodeBase[T, S], other *NodeBase[T, S]) {
	if root == nil || other == nil {
		return
	}

	// Merge the counters
	root.Stats.AddVvl(other.Stats.Visits(), other.Stats.VirtualLoss())
	root.Stats.AddOutcome(other.Stats.Outcomes())

	// Merge children
	otherLen := len(other.Children)
	rootLen := len(root.Children)

	// We have a mismatch, try to find the child
	if rootLen != otherLen {
		// Mismatch in number of children, cannot merge
		// because we don't know where to put the new child,
		// This will happen on 'almost' leaf nodes, so skipping them
		// is fine.
		if rootLen == 0 && otherLen != 0 {
			// If the root has no children, but the other has,
			// we can copy them all
			root.Children = make([]NodeBase[T, S], otherLen)
			copy(root.Children, other.Children)
		}
		return
	}

	// Merge children
	for i := 0; i < otherLen; i++ {
		child := &other.Children[i]

		// Assume children are ordered the same way
		if child.NodeSignature == root.Children[i].NodeSignature {
			mergeResult(&root.Children[i], child)
		} else {
			panic("[MCTS] mergeResult: child signature mismatch, make sure GameOperations.ExpandNode returns children ALWAYS in the same order")
		}

		// Else, that's an implementation of 'GameOperations' issue
		// ExpandNode() must be a pure function

	}
}

// Run multi-treaded search, to wait for the result, call Synchronize
func (mcts *MCTS[T, S, R]) SearchMultiThreaded(ops GameOperations[T, S, R]) {
	mcts.setupSearch()
	threads := max(1, mcts.Limiter.Limits().NThreads)
	VirtualLoss = 2

	// Create a slice of root nodes
	mcts.roots = make([]*NodeBase[T, S], threads)
	for i := 0; i < threads; i++ {
		if i == 0 || mcts.multithreadPolicy == MultithreadTreeParallel {
			// All threads will work on the same root node
			mcts.roots[i] = mcts.Root
		} else if mcts.multithreadPolicy == MultithreadRootParallel {
			// Each thread (apart from the main one) will have it's own copy of the root node
			mcts.roots[i] = mcts.Root.Clone()
		}
	}

	for id := range threads {
		mcts.wg.Add(1)

		// Start the search in a separate goroutine
		go mcts.Search(mcts.roots[id], ops.Clone(), id)
	}
}

func (mcts *MCTS[T, S, R]) shouldMerge() bool {
	return mcts.multithreadPolicy == MultithreadRootParallel && mcts.Limiter.Limits().NThreads > 1
}

// This function only sets the limits, resets the counters, and the stop flag
// doesn't actually start the search
func (mcts *MCTS[T, S, R]) setupSearch() {
	// Setup
	// mcts.timer.Movetime(mcts.Limiter.Limits.Movetime)
	// mcts.timer.Reset()
	mcts.Limiter.Reset()
	mcts.cps.Store(0)
	mcts.maxdepth.Store(0)
	mcts.merged.Store(false)
	// mcts.stop.Store(false)
}

// Actual search function implementation, simply calls:
//
// 1. selection - to choose the most promising node
//
// 2. rollout - to simulate the user-defined game, and get the result of a playout
//
// 3. backpropagate - to increment counters up to the root
//
// Until runs out of the allocated time, nodes, or memory.
// threadId must be unique, 0 meaning it's the main search threads with some privileges
func (mcts *MCTS[T, S, R]) Search(root *NodeBase[T, S], ops GameOperations[T, S, R], threadId int) {
	threadRand := rand.New(rand.NewSource(time.Now().UnixNano() + int64(threadId)))
	ops.SetRand(threadRand)

	if root.Terminal() || len(root.Children) == 0 {
		if threadId == 0 {
			mcts.invokeListener(mcts.listener.onStop)
		}
		mcts.wg.Done()
		return
	}

	var node *NodeBase[T, S]

	for mcts.Limiter.Ok(mcts.Size(), uint32(mcts.MaxDepth()), uint32(mcts.Cycles())) {

		// Choose the most promising node
		node = mcts.Selection(root, ops, threadRand, threadId)
		// Get the result of the rollout/playout
		mcts.strategy.Backpropagate(ops, node, ops.Rollout())

		// Increment cycle count and store the cps
		mcts.cycles.Add(1)
		mcts.cps.Store(uint32(mcts.Cycles()) * 1000 / mcts.Limiter.Elapsed())
		// Invoke the 'onCycle' listener
		if threadId == 0 && mcts.listener.onCycle != nil &&
			mcts.Root.Stats.Visits()%int32(mcts.listener.nCycles) == 0 {
			mcts.listener.onCycle(toListenerStats(mcts))
		}
	}

	// Evaluate the stop reason, only main thread will do this
	if threadId == 0 {
		mcts.Limiter.EvaluateStopReason(mcts.Size(), uint32(mcts.MaxDepth()), uint32(mcts.Cycles()))
	}

	// Synchronize all threads
	mcts.Limiter.Stop()

	// Make sure only 1 thread calls this
	if threadId == 0 {
		mcts.invokeListener(mcts.listener.onStop)
		mcts.wg.Done()

		// Wait for other threads to finish
		mcts.wg.Wait()
		// If we are in 'root parallel' mode, merge the results
		if mcts.shouldMerge() {
			mcts.mergeResults()
		}
	} else {
		mcts.wg.Done()
	}
}

// Selects next child to expand, by user-defined selection policy
func (mcts *MCTS[T, S, R]) Selection(root *NodeBase[T, S], ops GameOperations[T, S, R], threadRand *rand.Rand, threadId int) *NodeBase[T, S] {

	node := root
	depth := 0
	for node.Expanded() {
		node = mcts.selection_policy(node, root)
		ops.Traverse(node.NodeSignature)
		depth++

		// Apply virtual loss
		node.Stats.AddVvl(VirtualLoss, VirtualLoss)
	}

	// Add new children to this node, after finding leaf node
	if node.Stats.RealVisits() > 0 && !node.Terminal() {
		// Expand the node, only if needed (expand flag is 0)
		if mcts.Limiter.Expand() && node.CanExpand() {
			mcts.size.Add(ops.ExpandNode(node))
			// Now update it's state
			node.FinishExpanding()
		}

		// Currently expanding
		first := true
		for node.Expanding() {
			if first {
				// If this is the first time, increment the collision counter
				mcts.collisionCount.Add(1)
				first = false
			}
			runtime.Gosched()
		}

		// Already set
		if node.Expanded() {
			// Select child at random
			node = &node.Children[threadRand.Int31n(int32(len(node.Children)))]
			// Traverse to this child
			ops.Traverse(node.NodeSignature)
			depth++
			// Apply again virtual loss
			node.Stats.AddVvl(VirtualLoss, VirtualLoss)
		}
	}

	// Set the 'max depth'
	if threadId == 0 && depth >= 2 && depth > int(mcts.maxdepth.Load()) {
		// Fix: Allow only 1 thread (main) to change the 'maxdepth'
		mcts.maxdepth.Store(int32(depth))
		mcts.invokeListener(mcts.listener.onDepth)
	}

	// return the candidate
	return node
}
