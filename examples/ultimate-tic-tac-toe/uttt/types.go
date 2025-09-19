package uttt

import (
	"fmt"
	"math"
	"unsafe"
)

// Type defines for the position
type PieceType int8
type TurnType bool
type PosType uint8 // Also used as move representation
type BoardType [9][9]PieceType
type PositionState uint8
type EntryNodeType uint8

// Type defines for search/limits
type ScoreType uint8

// Mate values for Score type
const (
	ValueScore ScoreType = 0
	MateScore  ScoreType = 1
)

type EngineLine struct {
	Bestmove  PosType `json:"bestmove"`
	Value     int     `json:"eval"`
	ScoreType ScoreType
	Pv        []PosType
}

// Get the string representation of the value
func (s EngineLine) StringValue(turn TurnType, absValue bool) string {
	if s.ScoreType == MateScore {
		// we are winning
		if s.Value > 0 {
			return fmt.Sprintf("%d%cM", int(math.Abs(float64(s.Value))), turnToChar(turn))
		}
		// enemy is winning
		return fmt.Sprintf("%d%cM", int(math.Abs(float64(s.Value))), turnToChar(!turn))

	} else if s.Value == -1 {
		return "0.5"
	}

	v := s.Value
	if absValue && turn == CircleTurn {
		v = 100 - v
	}
	return fmt.Sprintf("%.2f", float32(v)/100.0)
}

// Struct holding information about the score value of the search
type SearchResult struct {
	Lines  []EngineLine
	Nodes  uint64
	Cps    uint32
	Depth  int
	Cycles int32
	Turn   TurnType
}

func (s SearchResult) String() string {
	if len(s.Lines) > 0 {
		return fmt.Sprintf("eval %s depth %d cps %d nodes %d cycles %d pv %v",
			s.Lines[0].StringValue(s.Turn, false), s.Depth, s.Cps, s.Nodes, s.Cycles, s.Lines[0].Pv)
	}

	return fmt.Sprintf("eval NaN depth %d cps %d nodes %d cycles %d pv empty",
		s.Depth, s.Cps, s.Nodes, s.Cycles)
}

func (s SearchResult) MainLine() (EngineLine, bool) {
	if len(s.Lines) > 0 {
		return s.Lines[0], true
	}
	return EngineLine{}, false
}

// Fast bool to int conversion
func _boolToInt(v bool) int {
	return int(*(*byte)(unsafe.Pointer(&v)))
}

// Enum for position
const (
	PosIllegal      PosType = 255
	PosIndexIllegal PosType = 15 // same as big/small index mask
)

const (
	PositionUnResolved PositionState = iota
	PositionDraw
	PositionCircleWon
	PositionCrossWon
)

// Enum for the piece type
const (
	PieceNone PieceType = iota
	PieceCircle
	PieceCross
)

// Enum for the turns
const (
	CircleTurn TurnType = false
	CrossTurn  TurnType = true
)

func turnToChar(turn TurnType) rune {
	if turn == CircleTurn {
		return 'o'
	}
	return 'x'
}

// Create piece from a rune
func PieceFromRune(square rune) PieceType {
	switch square {
	case 'x':
		return PieceCross
	case 'o':
		return PieceCircle
	default:
		return PieceNone
	}
}
