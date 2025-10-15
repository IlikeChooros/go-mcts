package mcts

import (
	"math/rand"
	"runtime"
)

// Use when started multi-threaded search and want it to synchronize with this thread
func (mcts *MCTS[T, S, R, O, A]) Synchronize() {
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

func (mcts *MCTS[T, S, R, O, A]) mergeResults() {
	for _, other := range mcts.roots[1:] {
		mergeResult(mcts.Root, other)
	}
	mcts.merged.Store(true)
	mcts.roots = nil
}

// Helper function to merge results from other root nodes into the main root
func mergeResult[T MoveLike, S NodeStatsLike[S]](root *NodeBase[T, S], other *NodeBase[T, S]) {
	if root == nil || other == nil {
		return
	}

	// Merge the counters
	root.Stats.AddVvl(other.Stats.N(), other.Stats.VirtualLoss())
	root.Stats.AddQ(other.Stats.Q())

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
		if child.Move == root.Children[i].Move {
			mergeResult(&root.Children[i], child)
		} else {
			// Else, that's an implementation of 'GameOperations' issue
			// ExpandNode() must be a pure function
			panic("[MCTS] mergeResult: child move mismatch, make sure GameOperations.ExpandNode returns children ALWAYS in the same order")
		}
	}
}

// Used for pre-mature termination of search
func (mcts *MCTS[T, S, R, O, A]) prematureCleanup() {
	mcts.Limiter.Stop()
	mcts.Limiter.EvaluateStopReason(mcts.Size(), uint32(mcts.MaxDepth()), uint32(mcts.Cycles()))
	mcts.invokeListener(mcts.listener.onStop, false)
}

// Run multi-treaded search, to wait for the result, call Synchronize
func (mcts *MCTS[T, S, R, O, A]) SearchMultiThreaded() {
	if mcts.Root.Terminal() {
		// OnStop must always be called, when search terminates
		mcts.prematureCleanup()
		return
	}

	mcts.setupSearch()
	threads := max(1, mcts.Limiter.Limits().NThreads)

	if !mcts.Root.Expanded() && mcts.tryExpandingWarn(mcts.Root) {
		// Root is terminal, but wasn't marked as such
		mcts.prematureCleanup()
		panic("[MCTS] SearchMultiThreaded: root node is not terminal, but ExpandNode returned no children, search aborted")
	}

	// Create a slice of root nodes
	mcts.roots = make([]*NodeBase[T, S], threads)
	for id := range mcts.roots {
		if id == mainThreadId || mcts.multithreadPolicy != MultithreadRootParallel {
			// All threads will work on the same root node
			mcts.roots[id] = mcts.Root
		} else {
			// Each thread (apart from the main one) will have it's own copy of the root node
			mcts.roots[id] = mcts.Root.Clone(nil)
		}
	}

	for id := range mcts.roots {
		mcts.wg.Add(1)

		// Start the search in a separate goroutine
		go mcts.Search(mcts.roots[id], mcts.ops.Clone(), id)
	}
}

func (mcts *MCTS[T, S, R, O, A]) shouldMerge() bool {
	return mcts.multithreadPolicy == MultithreadRootParallel && mcts.Limiter.Limits().NThreads > 1
}

// This function only sets the limits, resets the counters, and the stop flag
// doesn't actually start the search
func (mcts *MCTS[T, S, R, O, A]) setupSearch() {
	mcts.Limiter.Reset()
	mcts.cps.Store(0)
	mcts.cycles.Store(0)
	mcts.maxdepth.Store(0)
	mcts.merged.Store(false)
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
// threadId must be unique, 0 meaning it's the main search thread which will call the listeners
func (mcts *MCTS[T, S, R, O, A]) Search(root *NodeBase[T, S], ops O, threadId int) {
	threadRand := rand.New(rand.NewSource(SeedGeneratorFn() + int64(threadId)))

	// For random (light) playouts, set the random number generator
	if rg, ok := GameOperations[T, S, R, O](ops).(RandGameOperations[T, S, R, O]); ok {
		rg.SetRand(threadRand)
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
		if threadId == mainThreadId && mcts.listener.onCycle != nil &&
			mcts.Root.Stats.N()%int32(mcts.listener.nCycles) == 0 {
			mcts.invokeListener(mcts.listener.onCycle, true)
		}
	}

	// Evaluate the stop reason, only main thread will do this
	if threadId == mainThreadId {
		mcts.Limiter.EvaluateStopReason(mcts.Size(), uint32(mcts.MaxDepth()), uint32(mcts.Cycles()))
	}

	// Stop every search thread
	mcts.Limiter.Stop()

	// Make sure only 1 thread calls this
	if threadId == mainThreadId {
		// onStop is the only listener that is always called, even if the search was stopped
		mcts.invokeListener(mcts.listener.onStop, false)
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
func (mcts *MCTS[T, S, R, O, A]) Selection(root *NodeBase[T, S], ops O, threadRand *rand.Rand, threadId int) *NodeBase[T, S] {

	node := root
	depth := int32(0)
	for node.Expanded() {
		node = mcts.strategy.Select(node, root)
		ops.Traverse(node.Move)
		depth++

		// Apply virtual loss
		node.Stats.AddVvl(VirtualLoss, VirtualLoss)
	}

	// Add new children to this node, after finding leaf node
	if node.Stats.RealVisits() > 0 && !node.Terminal() {
		// Expand the node, only if needed (expand flag is 0)
		if mcts.Limiter.Expand() && node.CanExpand() {
			v := ops.ExpandNode(node)
			if len(node.Children) == 0 {
				// Allocation failed, this may happen even if ops.ExpandNode
				// is properly implemented, undo the expanding state
				node.CancelExpanding()
			} else {
				// Now update it's state
				node.FinishExpanding()
				mcts.size.Add(v)
			}
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
			node = &node.Children[threadRand.Int31()%int32(len(node.Children))]
			ops.Traverse(node.Move)
			depth++
			// Apply again virtual loss
			node.Stats.AddVvl(VirtualLoss, VirtualLoss)
		}
	}

	// Set the 'max depth'
	if mcts.maxdepth.CompareAndSwap(depth-1, depth) {
		mcts.maxdepth.Store(depth)
		if depth > 1 {
			mcts.invokeListener(mcts.listener.onDepth, true)
		}
	}

	// return the candidate
	return node
}
