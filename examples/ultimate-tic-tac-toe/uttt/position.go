package uttt

import "fmt"

// Constants

const (
	StartingPosition string = "9/9/9/9/9/9/9/9/9 x -"
)

// Main position struct
type Position struct {
	position         BoardType // 2d array of the pieces [bigIndex][smallIndex]
	bitboards        [2][9]uint
	bigPositionState [9]PositionState // Array of uint8's, where each one means, either cross, circle or no one won on that square
	stateList        *StateList       // history of the position (for MakeMove, UndoMove)
	nextBigIndex     PosType
	termination      Termination
	// hash             uint64 // Current hash of the position
}

// Create a heap-allocated, initialized Big Tic Tac Toe position
func NewPosition() *Position {
	pos := &Position{}
	pos.Init()
	return pos
}

// Make a deep copy of the position (has no shared memory with this object)
func (p *Position) Clone() Position {
	pos := Position{
		stateList:    NewStateList(),
		nextBigIndex: p.nextBigIndex,
		termination:  p.termination,
		// hash:         p.hash,
	}

	for i := range 9 {
		copy(pos.position[i][:], p.position[i][:])
		pos.bigPositionState[i] = p.bigPositionState[i]
	}
	for i := range 2 {
		for j := range 9 {
			pos.bitboards[i][j] = p.bitboards[i][j]
		}
	}

	copy(pos.stateList.list, p.stateList.list)
	return pos
}

// Initialize the position
func (p *Position) Init() {
	p.stateList = NewStateList()
	// p.hash = p.Hash()
	p.nextBigIndex = PosIndexIllegal
}

func (p *Position) Reset() {
	p.stateList.Clear()
	p.termination = TerminationNone
	// p.hash = 0
	p.nextBigIndex = PosIndexIllegal

	// Zero the board
	for i := range p.position {
		for j := range p.position[i] {
			p.position[i][j] = PieceNone
		}
	}

	// Zero the bigPositionState
	for i := range p.bigPositionState {
		p.bigPositionState[i] = PositionUnResolved
		p.bitboards[0][i] = 0
		p.bitboards[1][i] = 0
	}
}

// Convert given 'small square' with given 'ourPiece' parameter, into (our bitboard, enemy bitboard)
func toBitboards(square [9]PieceType, ourPiece PieceType) (bitboard, enemy_bitboard uint) {
	// Write whole board into a bitboard
	for i, v := range square {
		// Evaluate square table evaluation
		if v == ourPiece {
			bitboard |= (1 << i)
		} else if v != PieceNone {
			// Enemy
			enemy_bitboard |= (1 << i)
		}
	}

	return bitboard, enemy_bitboard
}

// Make sure bitboards represent the same position as the 2d arrays
func (p *Position) MatchBitboards() {
	for i, square := range p.position {
		p.bitboards[1][i], p.bitboards[0][i] = toBitboards(square, PieceCross)
	}
}

func (pos *Position) SetupBoardState() {
	// Check each small square, and set proper big square state
	pos.MatchBitboards()
	for i := range pos.position {
		if pos.bigPositionState[i] == PositionUnResolved {
			pos.bigPositionState[i] = _checkSquareTermination(
				pos.bitboards[1][i], pos.bitboards[0][i],
			)
		}
	}
	// Don't allow playing on terminated ttt board
	if pos.nextBigIndex != PosIndexIllegal &&
		pos.bigPositionState[pos.nextBigIndex] != PositionUnResolved {
		pos.nextBigIndex = PosIndexIllegal
	}

	// pos.hash = pos.Hash()
}

// Getters
func (b *Position) Position() BoardType {
	return b.position
}

func (b *Position) Turn() TurnType {
	return !b.stateList.Last().turn
}

func (p *Position) BigIndex() PosType {
	return p.nextBigIndex
}

// func (p *Position) _UpdateBigPosHash(state PositionState, bigIndex PosType) {
// 	// Update the hash
// 	idx := -1
// 	switch state {
// 	case PositionCrossWon:
// 		idx = 1
// 	case PositionCircleWon:
// 		idx = 0
// 	case PositionDraw:
// 		idx = 2
// 	}

// 	if idx != -1 {
// 		p.hash ^= _hashBigPosState[idx][bigIndex]
// 	}
// }

// Verifies legality of given move, then if it's valid, make's it on the board
func (p *Position) MakeLegalMove(move PosType) error {
	if !p.IsLegal(move) {
		return fmt.Errorf("Move %s is illegal, possible moves=[%s]", move.String(), p.GenerateMoves().String())
	}
	p.MakeMove(move)
	return nil
}

// Make a move on the position, switches the sides, and puts current piece
// on the position [bigIndex][smallIndex], accepts any move
func (p *Position) MakeMove(move PosType) {
	// UPDATE: Apparently we can't make a move inside a terminated position
	bigIndex := move.BigIndex()
	if p.termination != TerminationNone {
		return
	}

	smallIndex := move.SmallIndex()

	// Make sure the coordinates are correct
	if smallIndex > 8 || bigIndex > 8 {
		return
	}

	// Choose the piece, based on the current side to move
	piece := PieceCircle

	// Now it's Cross's turn
	if p.Turn() == CrossTurn {
		piece = PieceCross
	}

	posStateBefore := p.bigPositionState[bigIndex]
	index := _boolToInt(bool(p.Turn()))
	nextBigIndex := smallIndex

	// Put that piece on the position
	p.position[bigIndex][smallIndex] = piece
	p.bitboards[index][bigIndex] ^= (1 << smallIndex)

	// Update Big board state, by checking if the smaller board, we are making move on,
	// is terminated
	p.bigPositionState[bigIndex] = _checkSquareTermination(
		p.bitboards[1][bigIndex], p.bitboards[0][bigIndex],
	)

	// If opponent's move would be on terminated tic tac toe board,
	// allow every it to play on every board
	if p.bigPositionState[nextBigIndex] != PositionUnResolved {
		nextBigIndex = PosIndexIllegal
	}

	// Update hash
	// p.hash ^= _hashSmallBoard[index][bigIndex][smallIndex]
	// p.hash ^= _hashTurn

	// Remove previous big index hash
	// if p.nextBigIndex != PosIndexIllegal {
	// p.hash ^= _hashBigIndex[p.nextBigIndex]
	// }

	// Set new 'big index'
	// if nextBigIndex != PosIndexIllegal {
	// p.hash ^= _hashBigIndex[nextBigIndex]
	// }

	// Update the big position state hash
	// p._UpdateBigPosHash(p.bigPositionState[bigIndex], bigIndex)

	// Append new state
	p.stateList.Append(move, !p.stateList.Last().turn, posStateBefore, p.nextBigIndex)
	p.nextBigIndex = nextBigIndex // update nextBigIndex
}

// Undo last move, from the state list
func (p *Position) UndoMove() {
	if p.stateList.ValidSize() == 0 {
		return
	}

	// Get the coordiantes
	lastState := p.stateList.Last()
	smallIndex := lastState.move.SmallIndex()
	bigIndex := lastState.move.BigIndex()
	index := _boolToInt(bool(lastState.turn))

	// Remove that piece from it's square
	p.position[bigIndex][smallIndex] = PieceNone
	p.bitboards[index][bigIndex] ^= (1 << smallIndex)

	// Remove piece, turn and bigIndex hash
	// p.hash ^= _hashSmallBoard[index][bigIndex][smallIndex]
	// p.hash ^= _hashTurn

	// if p.nextBigIndex != PosIndexIllegal {
	// p.hash ^= _hashBigIndex[p.nextBigIndex]
	// }

	// If this move had changed the big position state, update the hash
	// (last move terminated that tic tac toe board, so we should undo the state hash)
	// if lastState.thisPositionState != p.bigPositionState[bigIndex] {
	// p._UpdateBigPosHash(p.bigPositionState[bigIndex], bigIndex)
	// }

	// Restore bigPositionState
	p.bigPositionState[bigIndex] = lastState.thisPositionState

	// Restore termination
	p.termination = TerminationNone

	// Restore current state
	p.nextBigIndex = lastState.prevBigIndex
	p.stateList.Remove()

	// Add previous big index hash
	// if p.nextBigIndex != PosIndexIllegal {
	// p.hash ^= _hashBigIndex[p.nextBigIndex]
	// }
}

// Get the 'big position state'
func (p *Position) BigPositionState() [9]PositionState {
	return p.bigPositionState
}

// Check if given move is legal
func (p *Position) IsLegal(move PosType) bool {

	bi, si := move.BigIndex(), move.SmallIndex()
	if p.BigIndex() != PosIndexIllegal && bi != PosType(p.BigIndex()) {
		return false
	}

	// Index out of range, board terminated, non-empty square or tic tac toe board is terminated
	if bi >= 9 || si >= 9 ||
		p.termination != TerminationNone ||
		p.position[bi][si] != PieceNone ||
		p.bigPositionState[bi] != PositionUnResolved {
		return false
	}

	return true
}
