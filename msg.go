package main

import "time"

const (
	TypeModeS uint8 = iota
	TypeModeAC
)

type Message struct {
	Msg         []uint8
	ICAO        []uint8
	DF          uint8
	Type        uint8
	ReceiptTime time.Time
}
