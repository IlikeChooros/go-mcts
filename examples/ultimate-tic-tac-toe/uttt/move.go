package uttt

import "strings"

const (
	_moveBigIndexMask   = 0b11110000
	_moveSmallIndexMask = 0b1111
)

type MoveList struct {
	moves [9 * 9]PosType
	size  uint8
}

// Make a new move list struct
func NewMoveList() *MoveList {
	return &MoveList{}
}

func ToMoveList(moves []PosType) *MoveList {
	mv := &MoveList{}
	copy(mv.moves[:], moves)
	mv.size = uint8(len(moves))
	return mv
}

// Create a move, based on big and small indexes
func MakeMove(bigIndex, smallIndex int) PosType {
	return PosType((smallIndex & _moveSmallIndexMask) | ((bigIndex << 4) & _moveBigIndexMask))
}

// Reset the movelist, simply sets the size to 0
func (ml *MoveList) Clear() {
	ml.size = 0
}

// Get the actual slice of valid moves
func (ml *MoveList) Slice() []PosType {
	return ml.moves[0:ml.size]
}

func (ml *MoveList) Size() int {
	return int(ml.size)
}

// Appends a new move to the list of moves
func (ml *MoveList) Append(bigIndex, smallIndex int) {
	// This is a critical function, don't use MakeMove here
	ml.moves[ml.size] = PosType((smallIndex & _moveSmallIndexMask) | ((bigIndex << 4) & _moveBigIndexMask))
	ml.size++
}

// Convert movelist into a string, uses move notation with space seperation
func (ml *MoveList) String() string {
	if ml.size == 0 {
		return "empty"
	}

	strMoves := make([]string, ml.size)
	for i, m := range ml.Slice() {
		strMoves[i] = m.String()
	}
	return strings.Join(strMoves, " ")
}

func (ml *MoveList) AppendMove(move PosType) {
	ml.moves[ml.size] = move
	ml.size++
}

// Get the big index of a move
func (pos PosType) BigIndex() PosType {
	return (pos & _moveBigIndexMask) >> 4
}

// Get the small index of tic tac toe board
func (pos PosType) SmallIndex() PosType {
	return pos & _moveSmallIndexMask
}

// Enum for the squares (same for the smaller ones)
const (
	A3 int = iota
	B3
	C3
	A2
	B2
	C2
	A1
	B1
	C1
)

// Get string representation of the move, will contain
// a/b/c 1/2/3 as coorinates, for example big index = 7,
// small index = 2 -> <big index part><small index part>
// -> B1c3
//
//	     	A    B    C
//			 0 | 1 | 2	3
//			-----------
//			 3 | 4 | 5	2
//			-----------
//		     6 | 7 | 8	1
func (pos PosType) String() string {
	builder := strings.Builder{}
	si, bi := pos.SmallIndex(), pos.BigIndex()

	if si >= 9 || bi >= 9 {
		return "(none)"
	}

	builder.WriteByte('A' + byte(bi%3))
	builder.WriteByte('3' - byte(bi/3))
	builder.WriteByte('a' + byte(si%3))
	builder.WriteByte('3' - byte(si/3))

	return builder.String()
}

// Convert given move notation (should be done with PosType.String()) to PosType
func MoveFromString(str string) PosType {
	if str == "(none)" || len(str) != 4 {
		return PosIllegal
	}

	// Helper function to make sure the coordinates are withing the range
	_cmp := func(i int, letter byte) bool {
		return (str[i] >= letter && str[i] <= letter+2) &&
			(str[i+1] >= '1' && str[i+1] <= '3')
	}

	if _cmp(0, 'A') && _cmp(2, 'a') {
		return MakeMove(
			int((str[0]-'A')+('3'-str[1])*3),
			int((str[2]-'a')+('3'-str[3])*3))
	}

	return PosIllegal
}
