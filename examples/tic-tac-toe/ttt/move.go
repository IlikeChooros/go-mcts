package ttt

// Enum for the squares (same for the smaller ones)
const (
	A3 PosType = iota
	B3
	C3
	A2
	B2
	C2
	A1
	B1
	C1
)

const (
	PosIllegal PosType = 255
)

type MoveList struct {
	Moves [9]PosType
	Size  uint8
}

func NewMoveList() *MoveList {
	return &MoveList{}
}

func (ml *MoveList) AppendMove(mv PosType) {
	ml.Moves[ml.Size] = mv
	ml.Size++
}
