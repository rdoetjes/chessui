//go:build js && wasm

package main

import (
	"fmt"
	"math"
	"syscall/js"

	"github.com/notnil/chess"
)

func getXY(file, rank int, tileSize float64) (float64, float64) {
	if playerColor == chess.White {
		return float64(file) * tileSize, float64(7-rank) * tileSize
	}
	return float64(7-file) * tileSize, float64(rank) * tileSize
}

func drawBoard() {
	now := js.Global().Get("performance").Call("now").Float()
	if animProgress < 1.0 {
		animProgress = (now - animStartTime) / animDuration
		if animProgress > 1.0 {
			animProgress = 1.0
		}
	}

	rect := boardCanvas.Call("getBoundingClientRect")
	w := rect.Get("width").Float()
	if w < 10 {
		w = 600
	}

	if boardCanvas.Get("width").Float() != w {
		boardCanvas.Set("width", w)
		boardCanvas.Set("height", w)
	}

	size, tileSize := w, w/8

	// Clear
	ctx.Set("fillStyle", "#000")
	ctx.Call("fillRect", 0, 0, size, size)

	// Draw squares and coordinates
	for screenY := 0; screenY < 8; screenY++ {
		for screenX := 0; screenX < 8; screenX++ {
			isLight := (screenX+screenY)%2 == 0
			if isLight {
				ctx.Set("fillStyle", "#b5b5b5") // Darker "light" square
			} else {
				ctx.Set("fillStyle", "#555555") // True dark charcoal
			}
			ctx.Call("fillRect", float64(screenX)*tileSize, float64(screenY)*tileSize, tileSize, tileSize)

			// Determine logical coords for labels
			fileIdx, rankIdx := screenX, 7-screenY
			if playerColor == chess.Black {
				fileIdx, rankIdx = 7-screenX, screenY
			}

			ctx.Set("font", "bold 12px Arial")
			if screenX == 0 {
				label := fmt.Sprintf("%d", rankIdx+1)
				if isLight {
					ctx.Set("fillStyle", "#555555")
				} else {
					ctx.Set("fillStyle", "#b5b5b5")
				}
				ctx.Call("fillText", label, 2.0, float64(screenY)*tileSize+12.0)
			}
			if screenY == 7 {
				label := string(rune('a' + fileIdx))
				if isLight {
					ctx.Set("fillStyle", "#555555")
				} else {
					ctx.Set("fillStyle", "#b5b5b5")
				}
				ctx.Call("fillText", label, float64(screenX+1)*tileSize-10.0, float64(screenY+1)*tileSize-4.0)
			}
		}
	}

	if lastMove != nil && !isSetupMode {
		ctx.Set("fillStyle", "rgba(255, 170, 0, 0.4)") // Pale Orange highlight
		x1, y1 := getXY(int(lastMove.S1().File()), int(lastMove.S1().Rank()), tileSize)
		x2, y2 := getXY(int(lastMove.S2().File()), int(lastMove.S2().Rank()), tileSize)
		ctx.Call("fillRect", x1, y1, tileSize, tileSize)
		ctx.Call("fillRect", x2, y2, tileSize, tileSize)
	}

	// Check / Checkmate highlight
	pos := game.Position()
	status := pos.Status()
	if !isSetupMode && (status == chess.Checkmate || (lastMove != nil && lastMove.HasTag(chess.Check))) {
		board := pos.Board()
		turn := pos.Turn()
		for i := 0; i < 64; i++ {
			sq := chess.Square(i)
			p := board.Piece(sq)
			if p.Type() == chess.King && p.Color() == turn {
				ctx.Set("fillStyle", "rgba(255, 0, 0, 0.6)") // Slightly stronger red for mate
				x, y := getXY(int(sq.File()), int(sq.Rank()), tileSize)
				ctx.Call("fillRect", x, y, tileSize, tileSize)
				break
			}
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

			// Skip drawing the piece if it's currently animating
			if animProgress < 1.0 && animMove != nil && sq == animMove.S2() {
				continue
			}

			if p != chess.NoPiece {
				drawPieceAt(sq, p, tileSize)
			}
		}

		// Draw the animating piece
		if animProgress < 1.0 && animMove != nil {
			x1, y1 := getXY(int(animMove.S1().File()), int(animMove.S1().Rank()), tileSize)
			x2, y2 := getXY(int(animMove.S2().File()), int(animMove.S2().Rank()), tileSize)

			currentX := x1 + (x2-x1)*animProgress
			currentY := y1 + (y2-y1)*animProgress

			drawPieceAtCoords(animPiece, currentX, currentY, tileSize)
		}
	}

	if selectedPos != chess.NoSquare && !isSetupMode {
		x, y := getXY(int(selectedPos.File()), int(selectedPos.Rank()), tileSize)
		ctx.Set("fillStyle", "rgba(0, 255, 0, 0.3)")
		ctx.Call("fillRect", x, y, tileSize, tileSize)
		ctx.Set("fillStyle", "rgba(255, 0, 0, 0.4)")
		for _, m := range game.ValidMoves() {
			if m.S1() == selectedPos {
				targetX, targetY := getXY(int(m.S2().File()), int(m.S2().Rank()), tileSize)
				ctx.Call("beginPath")
				ctx.Call("arc", targetX+tileSize/2, targetY+tileSize/2, tileSize*0.12, 0, 2*math.Pi)
				ctx.Call("fill")
			}
		}
	}

	// Game Over Overlay
	if isGameOver {
		ctx.Set("fillStyle", "rgba(0, 0, 0, 0.5)")
		ctx.Call("fillRect", 0, 0, size, size)

		ctx.Set("fillStyle", "white")
		ctx.Set("textAlign", "center")
		ctx.Set("font", "bold 60px Arial")

		msg := "GAME OVER"
		if game.Position().Status() == chess.Checkmate {
			msg = "CHECKMATE"
			ctx.Set("fillStyle", "#ff4444")
		}

		ctx.Call("fillText", msg, size/2, size/2)

		ctx.Set("font", "bold 24px Arial")
		ctx.Set("fillStyle", "white")
		ctx.Call("fillText", "Outcome: "+game.Outcome().String(), size/2, size/2+40)
	}
}

func drawPieceAt(sq chess.Square, p chess.Piece, tileSize float64) {
	x, y := getXY(int(sq.File()), int(sq.Rank()), tileSize)
	drawPieceAtCoords(p, x, y, tileSize)
}

func drawPieceAtCoords(p chess.Piece, x, y, tileSize float64) {
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
