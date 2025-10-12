package uttt

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestRandomPlayout(t *testing.T) {
	pos := NewPosition()

	for i := 0; i < 50000; i++ {
		t.Run(fmt.Sprintf("Playout-%d", i), func(t *testing.T) {
			movesLeft := 81
			p := pos.Clone()
			for !p.IsTerminated() && movesLeft > 0 {
				moves := p.GenerateMoves()
				if moves.Size == 0 {
					t.Fatal("No legal moves available")
				}
				move := moves.Slice()[rand.Int()%int(moves.Size)]
				p.MakeMove(move)
				movesLeft--
			}
			if p.Termination() == TerminationNone {
				t.Fatal("Game ended without a termination condition")
			}
		})
	}
}
