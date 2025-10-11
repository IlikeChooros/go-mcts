package chessmcts

import (
	"testing"

	mcts "github.com/IlikeChooros/go-mcts/pkg/mcts"
)

func BenchmarkRAVE(b *testing.B) {
	rave := NewRaveMcts()

	rave.Strategy().SetBetaFunction(func(playouts, playoutsContatingMove int32) float64 {
		const (
			b      = 0.1
			factor = 4 * b * b
		)
		return float64(playouts) / (float64(playouts+playoutsContatingMove) + factor*float64(playouts*playoutsContatingMove))
	})

	b.ResetTimer()
	for range b.N {
		rave.SetLimits(mcts.DefaultLimits().SetThreads(12).SetCycles(10000))
		rave.Search()
	}
}
