package ttt

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

// Check if given 'small' square is terminated
func (p *Position) CheckTerminationPattern() {
	crossbb := uint(p.bitboards[_bitboardCrossIdx])
	circlebb := uint(p.bitboards[_bitboardCircleIdx])

	// See if there iS NodeStatsLike winning patterns
	for i := range 8 {
		if crossbb&_winningBitboardPatterns[i] == _winningBitboardPatterns[i] {
			p.termination = TerminationCrossWon
			return
		}
		if circlebb&_winningBitboardPatterns[i] == _winningBitboardPatterns[i] {
			p.termination = TerminationCircleWon
			return
		}
	}

	// If not, check if that's a draw (this square is fully filled)
	if (crossbb | circlebb) == 0b111111111 {
		p.termination = TerminationDraw
	}
}
