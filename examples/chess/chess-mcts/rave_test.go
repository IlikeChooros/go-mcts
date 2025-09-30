package chessmcts

import (
	"testing"

	mcts "github.com/IlikeChooros/go-mcts/pkg/mcts"
)

func BenchmarkRAVE(b *testing.B) {
	ucb := NewRaveMcts()

	mcts.RaveBetaFunction = func(playouts, playoutsContatingMove int32) float64 {
		const (
			b      = 0.1
			factor = 4 * b * b
		)
		return float64(playouts) / (float64(playouts+playoutsContatingMove) + factor*float64(playouts*playoutsContatingMove))
	}

	b.ResetTimer()
	for range b.N {
		ucb.SetLimits(mcts.DefaultLimits().SetThreads(12).SetCycles(10000))
		ucb.Search()
	}
}
