package uttt

type Termination int

const (
	TerminationNone            Termination = 0
	TerminationCircleWon       Termination = 1
	TerminationCrossWon        Termination = 2
	TerminationDraw            Termination = 4
	TerminationResigned        Termination = 8
	TerminationIllegalPosition Termination = 16
)

// horizontal, vertical and diagonal patterns as bitboards
var _winningBitboardPatterns [8]uint = [...]uint{
	0b111000000, 0b000111000, 0b000000111,
	0b100100100, 0b010010010, 0b001001001,
	0b100010001, 0b001010100,
}

var _patterns = [8][3]int{
	{0, 1, 2}, {3, 4, 5}, {6, 7, 8},
	{0, 3, 6}, {1, 4, 7}, {2, 5, 8},
	{0, 4, 8}, {2, 4, 6},
}

// Set the termination flag
func (p *Position) SetTermination(t Termination) {
	p.termination = t
}

// Get the termination reason (after, calling IsTerminated, or CheckTerminationPattern)
func (p *Position) Termination() Termination {
	return p.termination
}

// Check if the whole board is terminated
func (p *Position) IsTerminated() bool {
	if p.termination != TerminationNone {
		return true
	}

	// Evaluate termination
	p.CheckTerminationPattern()
	return p.termination != TerminationNone
}

// Check if given slice is filled with items other than 'none'
func _isFilled[T comparable](arr []T, none T) bool {
	is_filled := true
	for i := 0; is_filled && i < len(arr); i++ {
		is_filled = arr[i] != none
	}
	return is_filled
}

// Check if given 'small' square is terminated
func _checkSquareTermination(crossbb, circlebb uint) PositionState {

	// See if there is any winning patterns
	for i := 0; i < 8; i++ {
		if crossbb&_winningBitboardPatterns[i] == _winningBitboardPatterns[i] {
			return PositionCrossWon
		}
		if circlebb&_winningBitboardPatterns[i] == _winningBitboardPatterns[i] {
			return PositionCircleWon
		}
	}

	// If not, check if that's a draw (this square is fully filled)
	if (crossbb | circlebb) == 0b111111111 {
		return PositionDraw
	}
	// Else, it's unresovled
	return PositionUnResolved
}

func (pos *Position) CheckTerminationPattern() {
	// Check if we are in a terminated state of the board
	// Assuming we correctly updated 'bigPositionState' (with the _checkSquareTermination)

	// Check winning conditions for all patterns
	// removed refrence by value, since that results in a copy of [3]int array, which slows down
	// this function by 100%
	for i := 0; i < 8; i++ {
		// Check this pattern, and resolve it
		if v := pos.bigPositionState[_patterns[i][0]]; v == pos.bigPositionState[_patterns[i][1]] &&
			pos.bigPositionState[_patterns[i][1]] == pos.bigPositionState[_patterns[i][2]] &&
			v != PositionUnResolved && v != PositionDraw {

			// Got a winner
			if v == PositionCircleWon {
				pos.termination = TerminationCircleWon
			} else {
				pos.termination = TerminationCrossWon
			}
			return // Exit the function
		}
	}

	// Check other draw condition
	// If there is no winner, check if all of the squares position
	// aren't unresolved, if so that means we got a draw
	is_draw := _isFilled(pos.bigPositionState[:], PositionUnResolved)
	if is_draw {
		pos.termination = TerminationDraw
	} else {
		// If our current BigIndex board is terminated, this means we got a
		// setup board position and the user incorrecly set the BigIndex, thus there is no
		// possible move to make
		if bi := pos.BigIndex(); bi != PosIndexIllegal && pos.bigPositionState[bi] != PositionUnResolved {
			pos.termination = TerminationIllegalPosition
		} else {
			// This is neither a draw or a win, so there is no termination
			pos.termination = TerminationNone
		}
	}
}
