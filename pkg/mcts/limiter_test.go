package mcts

import (
	"testing"
	"time"
)

func TestLimiterSingleLimits(t *testing.T) {
	limiter := LimiterLike(NewLimiter(32))

	if !limiter.Ok(1000000, 1000000, 1, 1) || !limiter.Expand() {
		t.Error("Default limiter should search infinitely, expand=", limiter.Expand())
	}

	limiter.SetLimits(DefaultLimits().SetNodes(100))
	limiter.Reset()
	if ok := limiter.Ok(101, 1, 1, 1); ok {
		t.Errorf("<Nodes=%d: ok=%v, want=%v", 101, ok, !ok)
	}

	if ok := limiter.Ok(99, 1, 1, 1); !ok {
		t.Errorf(">Nodes=%d: ok=%v, want=%v", 99, ok, !ok)
	}

	limiter.SetLimits(DefaultLimits().SetByteSize(10 * 32))
	limiter.Reset()

	if ok := limiter.Ok(1, 10, 1, 1); ok {
		t.Errorf("<Size=%d: ok=%v, want=%v", 10, ok, !ok)
	}

	if ok := limiter.Ok(99, 9, 1, 1); !ok {
		t.Errorf(">Size=%d: ok=%v, want=%v", 9, ok, !ok)
	}

	limiter.SetLimits(DefaultLimits().SetMovetime(100))
	limiter.Reset()
	time.Sleep(time.Millisecond * 101)

	if ok := limiter.Ok(1, 1, 1, 1); ok {
		t.Errorf("<Movetime: ok=%v, want=%v", ok, !ok)
	}

	limiter.Reset()
	if ok := limiter.Ok(1, 1, 1, 1); !ok {
		t.Errorf(">Movetime: ok=%v, want=%v", ok, !ok)
	}
}

func TestLimiterCombos(t *testing.T) {

	limiter := LimiterLike(NewLimiter(32))

	// Set different limits
	limiter.SetLimits(DefaultLimits().SetNodes(100).SetByteSize(32 * 10))
	limiter.Reset()

	// Nodes + memory limit, if memory exhausted, wait for 'nodes' and disable expanding
	if !(limiter.Ok(99, 10, 1, 1) && !limiter.Expand()) {
		t.Error(">Nodes+Memory failed: ok=", limiter.Ok(99, 10, 1, 1), "expand=", limiter.Expand())
	}
	if !(!limiter.Ok(101, 10, 1, 1) && !limiter.Expand()) {
		t.Error("<Nodes+Memory failed: ok=", limiter.Ok(101, 10, 1, 1), "expand=", limiter.Expand())
	}

	// Time + memory limit
	limiter.SetLimits(DefaultLimits().SetMovetime(100).SetByteSize(32 * 10))
	limiter.Reset()

	if !(limiter.Ok(1, 100, 1, 1) && !limiter.Expand()) {
		t.Error(">Time+Memory failed: ok=", limiter.Ok(1, 100, 1, 1), "expand=", limiter.Expand())
	}

	time.Sleep(time.Millisecond * 101)
	if !(!limiter.Ok(1, 100, 1, 1) && !limiter.Expand()) {
		t.Error("<Time+Memory failed: ok=", limiter.Ok(1, 100, 1, 1), "expand=", limiter.Expand())
	}
}
