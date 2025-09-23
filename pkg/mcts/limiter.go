package mcts

import (
	"context"
	"math"
	"sync/atomic"
	"unsafe"
)

type StopReason int

const (
	StopNone      StopReason = iota
	StopInterrupt            = 1  // Stopped by user, by calling .SetStop(true) or context cancellation
	StopMovetime             = 2  // Time limit reached
	StopMemory               = 4  // Memory limit reached
	StopDepth                = 8  // Depth limit reached
	StopCycles               = 16 // Cycle limit reached
)

func (sr StopReason) String() string {
	if sr == StopNone {
		return "None"
	}

	reasons := []struct {
		flag StopReason
		name string
	}{
		{StopInterrupt, "Interrupt"},
		{StopMovetime, "Movetime"},
		{StopMemory, "Memory"},
		{StopDepth, "Depth"},
		{StopCycles, "Cycles"},
	}

	var result string
	for _, r := range reasons {
		if sr&r.flag == r.flag {
			if result != "" {
				result += "|"
			}
			result += r.name
		}
	}

	return result
}

const (
	stopMask   int = StopInterrupt
	timeMask   int = StopMovetime
	memoryMask int = StopMemory
	depthMask  int = StopDepth
	cyclesMask int = StopCycles
)

type LimiterLike interface {
	SetContext(ctx context.Context)
	// Set the limits
	SetLimits(*Limits)
	// Get the limits
	Limits() *Limits
	// Get elapsed time in ms (from the last 'Reset' call)
	Elapsed() uint32
	// Set the stop signal, will cause to exit search if set to true
	SetStop(bool)
	// Get the stop signal
	Stop() bool
	// Reset the limiter's flags, called on search setup
	Reset()
	// Wheter the tree can grow
	Expand() bool
	// Wheter the search should stop, called in the main search loop
	Ok(size, depth, cycles uint32) bool
	// Get the reason why the search was stopped, valid after search ends
	StopReason() StopReason
	// Evaluate stop reason based on current state, and set it internally,
	// this will be called once (by main thread) after search ends, before synchronizing other unfinished threads
	EvaluateStopReason(size, depth, cycles uint32)
}

type Limiter struct {
	limits     *Limits
	Timer      *_Timer
	nodeSize   uint32
	maxSize    uint32
	expand     atomic.Bool
	stop       atomic.Bool
	areSetMask int
	reason     StopReason
	ctx        context.Context
}

func NewLimiter(nodesize uint32) *Limiter {
	limiter := &Limiter{
		limits:   DefaultLimits(),
		Timer:    _NewTimer(),
		nodeSize: nodesize,
		ctx:      context.Background(),
	}

	limiter.expand.Store(true)
	return limiter
}

func (l *Limiter) Reset() {
	l.Timer.Movetime(l.limits.Movetime)
	l.Timer.Reset()
	l.stop.Store(false)
	l.expand.Store(true)
	l.reason = StopNone

	// Calculate 'nodes' based on memory
	if l.limits.ByteSize != DefaultByteSizeLimit {
		l.maxSize = uint32(l.limits.ByteSize) / l.nodeSize
	} else {
		l.maxSize = math.MaxUint32
	}

	// Pre-calculate 'are set' limit mask, see 'Ok' method for more explanation
	l.areSetMask = toMask(l.Timer.IsSet(), 1) |
		toMask(l.limits.ByteSize != DefaultByteSizeLimit, 2) |
		toMask(l.limits.Depth != DefaultDepthLimit, 3) |
		toMask(l.limits.Cycles != DefaultCyclesLimit, 4)
}

func (l *Limiter) EvaluateStopReason(size, depth, cycles uint32) {
	okMask := l.OkMask(size, depth, cycles)
	reason := StopNone

	if okMask&stopMask == stopMask {
		reason |= StopInterrupt
	}

	if okMask&timeMask == timeMask {
		reason |= StopMovetime
	}

	if okMask&memoryMask == memoryMask {
		reason |= StopMemory
	}

	if okMask&depthMask == depthMask {
		reason |= StopDepth
	}

	if okMask&cyclesMask == cyclesMask {
		reason |= StopCycles
	}

	l.reason = reason
}

func (l *Limiter) StopReason() StopReason {
	return l.reason
}

func (l *Limiter) SetContext(ctx context.Context) {
	l.ctx = ctx
}

func (l *Limiter) SetStop(v bool) {
	l.stop.Store(v)
}

func (l *Limiter) Stop() bool {
	select {
	case <-l.ctx.Done():
		l.stop.Store(true)
	default:
	}
	return l.stop.Load()
}

func (l *Limiter) SetLimits(limits *Limits) {
	l.limits = limits
}

func (l *Limiter) Limits() *Limits {
	return l.limits
}

func (l *Limiter) Elapsed() uint32 {
	return uint32(l.Timer.Deltatime())
}

func (l *Limiter) Expand() bool {
	return l.expand.Load()
}

func toMask(val bool, offset int) int {
	return int(*(*byte)(unsafe.Pointer(&val))) << offset
}

func (l *Limiter) LimitMask(size, depth, cycles uint32) int {
	stop := l.Stop()
	// If infinite, always return 0 (no limits reached)
	if l.limits.Infinite {
		return toMask(stop, 0)
	}

	limitMask := 0

	limitMask |= toMask(stop, 0)
	limitMask |= toMask(l.Timer.IsEnd(), 1)
	limitMask |= toMask(l.maxSize <= size, 2)
	limitMask |= toMask(l.limits.Depth <= int(depth), 3)
	limitMask |= toMask(l.limits.Cycles <= cycles, 4)

	return limitMask
}

func (l *Limiter) OkMask(size, depth, cycles uint32) int {
	limitMask := l.LimitMask(size, depth, cycles)

	// Hierachy of stop signals
	// 1. stop
	// 2. Movetime
	// 3. Memory
	// 4. Depth

	// Check the combos:
	// (time/nodes/cycles or any combination of them) AND memory limit ->
	// if memory is exhausted, disable expanding of the tree and wait for the other limitation/s
	if (l.areSetMask&memoryMask) == memoryMask && (l.areSetMask&(timeMask|cyclesMask)) != 0 {
		// Memory exhausted
		if limitMask&memoryMask == memoryMask {
			l.expand.Store(false)
			limitMask ^= memoryMask // remove memory limitation
		}
	}

	return limitMask
}

func (l *Limiter) Ok(size, depth, cycles uint32) bool {
	return l.OkMask(size, depth, cycles) == 0
}
