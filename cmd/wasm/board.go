//go:build js && wasm

package main

import (
	"math"

	"github.com/notnil/chess"
)

func getXY(file, rank int, tileSize float64) (float64, float64) {
	if playerColor == chess.White {
		return float64(file) * tileSize, float64(7-rank) * tileSize
	}
	// Flip for Black
	return float64(7-file) * tileSize, float64(rank) * tileSize
}

func drawBoard() {
	rect := boardCanvas.Call("getBoundingClientRect")
	w := rect.Get("width").Float()
	if w < 10 {
		w = 600
	}

	if boardCanvas.Get("width").Float() != w {
		boardCanvas.Set("width", w)
		boardCanvas.Set("height", w)
	}

	size := w
	tileSize := size / 8

	// Clear
	ctx.Set("fillStyle", "#000")
	ctx.Call("fillRect", 0, 0, size, size)

	// Draw squares
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			// Logical coordinates for square color
			if (x+y)%2 != 0 {
				ctx.Set("fillStyle", "#d1d1d1") // Light Grey
			} else {
				ctx.Set("fillStyle", "#777777") // Medium Grey
			}
			ctx.Call("fillRect", float64(x)*tileSize, float64(y)*tileSize, tileSize, tileSize)
		}
	}

	if isSetupMode {
		for sq, p := range setupMap {
			drawPieceAt(sq, p, tileSize)
		}
	} else {
		board := game.Position().Board()
		for i := 0; i < 64; i++ {
			sq := chess.Square(i)
			p := board.Piece(sq)
			if p != chess.NoPiece {
				drawPieceAt(sq, p, tileSize)
			}
		}
	}

	// Move highlights
	if lastMove != nil && !isSetupMode {
		ctx.Set("fillStyle", "rgba(255, 255, 0, 0.2)")
		x1, y1 := getXY(int(lastMove.S1().File()), int(lastMove.S1().Rank()), tileSize)
		x2, y2 := getXY(int(lastMove.S2().File()), int(lastMove.S2().Rank()), tileSize)
		ctx.Call("fillRect", x1, y1, tileSize, tileSize)
		ctx.Call("fillRect", x2, y2, tileSize, tileSize)
	}

	// Legal move dots
	if selectedPos != chess.NoSquare && !isSetupMode {
		x, y := getXY(int(selectedPos.File()), int(selectedPos.Rank()), tileSize)
		ctx.Set("fillStyle", "rgba(0, 255, 0, 0.3)")
		ctx.Call("fillRect", x, y, tileSize, tileSize)

		ctx.Set("fillStyle", "rgba(255, 0, 0, 0.4)")
		for _, m := range game.ValidMoves() {
			if m.S1() == selectedPos {
				s2 := m.S2()
				targetX, targetY := getXY(int(s2.File()), int(s2.Rank()), tileSize)
				centerX := targetX + tileSize/2
				centerY := targetY + tileSize/2
				ctx.Call("beginPath")
				ctx.Call("arc", centerX, centerY, tileSize*0.15, 0, 2*math.Pi)
				ctx.Call("fill")
			}
		}
	}
}

func drawPieceAt(sq chess.Square, p chess.Piece, tileSize float64) {
	pStr, pChar := "w", ""
	if p.Color() == chess.Black {
		pStr = "b"
	}
	switch p.Type() {
	case chess.Pawn:
		pChar = "p"
	case chess.Knight:
		pChar = "n"
	case chess.Bishop:
		pChar = "b"
	case chess.Rook:
		pChar = "r"
	case chess.Queen:
		pChar = "q"
	case chess.King:
		pChar = "k"
	}
	pStr += pChar

	x, y := getXY(int(sq.File()), int(sq.Rank()), tileSize)

	if img, ok := images[pStr]; ok && img.Get("complete").Bool() && img.Get("naturalWidth").Int() > 0 {
		ctx.Call("drawImage", img, x, y, tileSize, tileSize)
	} else {
		ctx.Set("fillStyle", "white")
		if p.Color() == chess.Black {
			ctx.Set("fillStyle", "#aaa")
		}
		ctx.Set("font", "bold 30px Arial")
		ctx.Call("fillText", pStr, x+tileSize/4, y+tileSize/1.5)
	}
}
