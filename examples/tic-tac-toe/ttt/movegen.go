package ttt

import "math/bits"

func (p Position) GenerateMoves() *MoveList {
	movelist := NewMoveList()

	free := uint(0b111111111 ^ (p.bitboards[0] | p.bitboards[1]))
	for free != 0 {
		movelist.AppendMove(PosType(bits.TrailingZeros(free)))
		free &= free - 1
	}

	return movelist
}
