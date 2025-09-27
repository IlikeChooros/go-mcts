package ttt

type PosType uint8
type TurnType bool
type PlayerType uint8

const (
	CrossTurn  TurnType = true
	CircleTurn TurnType = false
)

const (
	None   PlayerType = 0
	Cross  PlayerType = 1
	Circle PlayerType = 2
)
