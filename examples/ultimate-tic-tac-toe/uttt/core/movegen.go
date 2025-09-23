package uttt

import (
	"math/bits"
)

// Generate all possible moves in given position
func (pos *Position) GenerateMoves() *MoveList {
	movelist := NewMoveList()
	free := uint(0)

	// If there is no history, we can choose also the 'Big Index' position
	if pos.BigIndex() == PosIndexIllegal {
		for bigIndex := 0; bigIndex < 9; bigIndex++ {
			if pos.bigPositionState[bigIndex] != PositionUnResolved {
				continue
			}

			// This is valid, because these 2 bitboards are mutally exclusive
			free = 0b111111111 ^ (pos.bitboards[0][bigIndex] | pos.bitboards[1][bigIndex])
			for free != 0 {
				movelist.Append(bigIndex, bits.TrailingZeros(free))
				free &= free - 1
			}
		}
	} else {
		// Else we generate moves for the 'Big Index' position
		bi := pos.BigIndex()
		if pos.bigPositionState[bi] != PositionUnResolved {
			return movelist
		}

		free = 0b111111111 ^ (pos.bitboards[0][bi] | pos.bitboards[1][bi])
		for free != 0 {
			movelist.Append(int(bi), bits.TrailingZeros(free))
			free &= free - 1
		}
	}

	return movelist
}
