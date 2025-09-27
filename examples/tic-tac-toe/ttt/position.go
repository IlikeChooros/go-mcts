package ttt

const (
	_bitboardCrossIdx  = 0
	_bitboardCircleIdx = 1
)

type Position struct {
	board       [9]PlayerType
	bitboards   [2]uint16
	history     []HistoryState
	termination Termination
}

func NewPosition() *Position {
	history := make([]HistoryState, 1, 4)
	history[0] = HistoryState{lastMove: PosIllegal, turn: !CrossTurn}

	return &Position{
		history: history,
	}
}

func (p *Position) lastHistory() *HistoryState {
	return &p.history[len(p.history)-1]
}

func (p *Position) Turn() TurnType {
	return !p.lastHistory().turn
}

func (p *Position) MakeMove(mv PosType) {
	// Allow every move
	idx := _bitboardCrossIdx
	player := Cross
	if p.Turn() == CircleTurn {
		player = Circle
		idx = _bitboardCircleIdx
	}

	p.bitboards[idx] ^= (1 << mv)
	p.board[mv] = player
	p.history = append(p.history, HistoryState{turn: !p.Turn(), lastMove: mv})
}

func (p *Position) UndoMove() {
	if len(p.history) <= 1 {
		return
	}

	idx := _bitboardCrossIdx
	if p.Turn() == CircleTurn {
		idx = _bitboardCircleIdx
	}

	hist := p.lastHistory()
	p.bitboards[idx] ^= (1 << hist.lastMove)
	p.board[hist.lastMove] = None
	p.termination = TerminationNone
	p.history = p.history[:len(p.history)-1]
}
