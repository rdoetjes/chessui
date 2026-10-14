//go:build js && wasm

package main

import (
	"fmt"
	"strings"

	"github.com/notnil/chess"
)

func generateFEN(m map[chess.Square]chess.Piece, turn string) string {
	fen := ""
	for r := 7; r >= 0; r-- {
		empty := 0
		for f := 0; f < 8; f++ {
			sq := chess.Square(r*8 + f)
			p, ok := m[sq]
			if !ok {
				empty++
			} else {
				if empty > 0 {
					fen += fmt.Sprintf("%d", empty)
					empty = 0
				}
				char := ""
				switch p.Type() {
				case chess.Pawn:
					char = "p"
				case chess.Knight:
					char = "n"
				case chess.Bishop:
					char = "b"
				case chess.Rook:
					char = "r"
				case chess.Queen:
					char = "q"
				case chess.King:
					char = "k"
				}
				if p.Color() == chess.White {
					char = strings.ToUpper(char)
				}
				fen += char
			}
		}
		if empty > 0 {
			fen += fmt.Sprintf("%d", empty)
		}
		if r > 0 {
			fen += "/"
		}
	}
	fen += fmt.Sprintf(" %s - - 0 1", turn)
	return fen
}
