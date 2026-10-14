//go:build js && wasm

package main

import (
	"math"

	"github.com/notnil/chess"
)

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

	// Draw squares (Neutral High Contrast Fallback)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
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
		ctx.Call("fillRect", float64(lastMove.S1().File())*tileSize, float64(7-lastMove.S1().Rank())*tileSize, tileSize, tileSize)
		ctx.Call("fillRect", float64(lastMove.S2().File())*tileSize, float64(7-lastMove.S2().Rank())*tileSize, tileSize, tileSize)
	}

	// Legal move dots
	if selectedPos != chess.NoSquare && !isSetupMode {
		ctx.Set("fillStyle", "rgba(0, 255, 0, 0.3)")
		ctx.Call("fillRect", float64(selectedPos.File())*tileSize, float64(7-selectedPos.Rank())*tileSize, tileSize, tileSize)

		ctx.Set("fillStyle", "rgba(255, 0, 0, 0.4)")
		for _, m := range game.ValidMoves() {
			if m.S1() == selectedPos {
				s2 := m.S2()
				centerX := float64(s2.File())*tileSize + tileSize/2
				centerY := float64(7-s2.Rank())*tileSize + tileSize/2
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

	if img, ok := images[pStr]; ok && img.Get("complete").Bool() && img.Get("naturalWidth").Int() > 0 {
		ctx.Call("drawImage", img, float64(sq.File())*tileSize, float64(7-sq.Rank())*tileSize, tileSize, tileSize)
	} else {
		ctx.Set("fillStyle", "white")
		if p.Color() == chess.Black {
			ctx.Set("fillStyle", "#aaa")
		}
		ctx.Set("font", "bold 30px Arial")
		ctx.Call("fillText", pStr, float64(sq.File())*tileSize+tileSize/4, float64(7-sq.Rank())*tileSize+tileSize/1.5)
	}
}
