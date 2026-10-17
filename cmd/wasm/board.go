//go:build js && wasm

package main

import (
	"fmt"
	"math"
	"syscall/js"

	"github.com/notnil/chess"
)

func getXY(file, rank int, tileSize float64) (float64, float64) {
	if app.multi.playerColor == chess.White {
		return float64(file) * tileSize, float64(7-rank) * tileSize
	}
	return float64(7-file) * tileSize, float64(rank) * tileSize
}

func drawBoard() {
	now := js.Global().Get("performance").Call("now").Float()
	if app.anim.progress < 1.0 {
		app.anim.progress = (now - app.anim.startTime) / app.anim.duration
		if app.anim.progress > 1.0 {
			app.anim.progress = 1.0
		}
	}

	rect := app.ui.boardCanvas.Call("getBoundingClientRect")
	w := rect.Get("width").Float()
	if w < 10 {
		w = 600
	}

	if app.ui.boardCanvas.Get("width").Float() != w {
		app.ui.boardCanvas.Set("width", w)
		app.ui.boardCanvas.Set("height", w)
	}

	size, tileSize := w, w/8

	// 1. Draw the board background image
	if img, ok := app.ui.images["board"]; ok && img.Get("complete").Bool() && img.Get("naturalWidth").Int() > 0 {
		app.ui.ctx.Call("drawImage", img, 0, 0, size, size)
	} else {
		// Fallback to manual drawing if image not loaded
		for screenY := 0; screenY < 8; screenY++ {
			for screenX := 0; screenX < 8; screenX++ {
				isLight := (screenX+screenY)%2 == 0
				if isLight {
					app.ui.ctx.Set("fillStyle", "#b5b5b5")
				} else {
					app.ui.ctx.Set("fillStyle", "#555555")
				}
				app.ui.ctx.Call("fillRect", float64(screenX)*tileSize, float64(screenY)*tileSize, tileSize, tileSize)
			}
		}
	}

	// 2. Draw coordinates
	for screenY := 0; screenY < 8; screenY++ {
		for screenX := 0; screenX < 8; screenX++ {
			isLight := (screenX+screenY)%2 == 0

			// Determine logical coords for labels
			fileIdx, rankIdx := screenX, 7-screenY
			if app.multi.playerColor == chess.Black {
				fileIdx, rankIdx = 7-screenX, screenY
			}

			app.ui.ctx.Set("font", "bold 12px Arial")

			// Handle coordinate colors based on theme
			coordColorLight := "#555555"
			coordColorDark := "#b5b5b5"

			if app.ui.currentStyle == "modern" {
				coordColorLight = "#ff3d3d"
				coordColorDark = "#ff3d3d"
			} else if app.ui.currentStyle == "classic" {
				coordColorLight = "#3d2b1f"
				coordColorDark = "#f5deb3"
			}

			if screenX == 0 {
				label := fmt.Sprintf("%d", rankIdx+1)
				if isLight {
					app.ui.ctx.Set("fillStyle", coordColorLight)
				} else {
					app.ui.ctx.Set("fillStyle", coordColorDark)
				}
				app.ui.ctx.Call("fillText", label, 2.0, float64(screenY)*tileSize+12.0)
			}
			if screenY == 7 {
				label := string(rune('a' + fileIdx))
				if isLight {
					app.ui.ctx.Set("fillStyle", coordColorLight)
				} else {
					app.ui.ctx.Set("fillStyle", coordColorDark)
				}
				app.ui.ctx.Call("fillText", label, float64(screenX+1)*tileSize-10.0, float64(screenY+1)*tileSize-4.0)
			}
		}
	}

	viewGame := getViewGame()
	viewPos := viewGame.Position()
	lastMoveInView := app.game.lastMove
	if app.game.currentViewIndex != -1 && app.game.currentViewIndex < len(app.game.game.Moves()) {
		lastMoveInView = app.game.game.Moves()[app.game.currentViewIndex]
	}

	if lastMoveInView != nil && !app.game.isSetupMode {
		app.ui.ctx.Set("fillStyle", "rgba(255, 170, 0, 0.4)") // Pale Orange highlight
		x1, y1 := getXY(int(lastMoveInView.S1().File()), int(lastMoveInView.S1().Rank()), tileSize)
		x2, y2 := getXY(int(lastMoveInView.S2().File()), int(lastMoveInView.S2().Rank()), tileSize)
		app.ui.ctx.Call("fillRect", x1, y1, tileSize, tileSize)
		app.ui.ctx.Call("fillRect", x2, y2, tileSize, tileSize)
	}

	// Check / Checkmate highlight
	status := viewPos.Status()
	if !app.game.isSetupMode && (status == chess.Checkmate || (lastMoveInView != nil && lastMoveInView.HasTag(chess.Check))) {
		board := viewPos.Board()
		turn := viewPos.Turn()
		for i := 0; i < 64; i++ {
			sq := chess.Square(i)
			p := board.Piece(sq)
			if p.Type() == chess.King && p.Color() == turn {
				app.ui.ctx.Set("fillStyle", "rgba(255, 0, 0, 0.6)") // Slightly stronger red for mate
				x, y := getXY(int(sq.File()), int(sq.Rank()), tileSize)
				app.ui.ctx.Call("fillRect", x, y, tileSize, tileSize)
				break
			}
		}
	}

	if app.game.isSetupMode {
		for sq, p := range app.game.setupMap {
			drawPieceAt(sq, p, tileSize)
		}
	} else if app.game.isStarted {
		board := viewPos.Board()
		for i := 0; i < 64; i++ {
			sq := chess.Square(i)
			p := board.Piece(sq)

			// Skip drawing the piece if it's currently animating (only if viewing latest)
			if app.game.currentViewIndex == -1 && app.anim.progress < 1.0 && app.anim.move != nil && sq == app.anim.move.S2() {
				continue
			}

			if p != chess.NoPiece {
				drawPieceAt(sq, p, tileSize)
			}
		}

		// Draw the animating piece (only if viewing latest)
		if app.game.currentViewIndex == -1 && app.anim.progress < 1.0 && app.anim.move != nil {
			x1, y1 := getXY(int(app.anim.move.S1().File()), int(app.anim.move.S1().Rank()), tileSize)
			x2, y2 := getXY(int(app.anim.move.S2().File()), int(app.anim.move.S2().Rank()), tileSize)

			currentX := x1 + (x2-x1)*app.anim.progress
			currentY := y1 + (y2-y1)*app.anim.progress

			drawPieceAtCoords(app.anim.piece, currentX, currentY, tileSize)
		}
	}

	if app.game.selectedPos != chess.NoSquare && !app.game.isSetupMode {
		x, y := getXY(int(app.game.selectedPos.File()), int(app.game.selectedPos.Rank()), tileSize)
		app.ui.ctx.Set("fillStyle", "rgba(0, 255, 0, 0.3)")
		app.ui.ctx.Call("fillRect", x, y, tileSize, tileSize)
		app.ui.ctx.Set("fillStyle", "rgba(255, 0, 0, 0.4)")
		for _, m := range viewGame.ValidMoves() {
			if m.S1() == app.game.selectedPos {
				targetX, targetY := getXY(int(m.S2().File()), int(m.S2().Rank()), tileSize)
				app.ui.ctx.Call("beginPath")
				app.ui.ctx.Call("arc", targetX+tileSize/2, targetY+tileSize/2, tileSize*0.12, 0, 2*math.Pi)
				app.ui.ctx.Call("fill")
			}
		}
	}

	// Game Over Overlay
	if app.game.isGameOver && app.game.currentViewIndex == -1 {
		app.ui.ctx.Set("fillStyle", "rgba(0, 0, 0, 0.5)")
		app.ui.ctx.Call("fillRect", 0, 0, size, size)

		app.ui.ctx.Set("fillStyle", "white")
		app.ui.ctx.Set("textAlign", "center")
		app.ui.ctx.Set("font", "bold 60px Arial")

		msg := "GAME OVER"
		if viewPos.Status() == chess.Checkmate {
			msg = "CHECKMATE"
			app.ui.ctx.Set("fillStyle", "#ff4444")
		}

		app.ui.ctx.Call("fillText", msg, size/2, size/2)

		app.ui.ctx.Set("font", "bold 24px Arial")
		app.ui.ctx.Set("fillStyle", "white")
		app.ui.ctx.Call("fillText", "Outcome: "+app.game.game.Outcome().String(), size/2, size/2+40)
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
	if img, ok := app.ui.images[pStr]; ok && img.Get("complete").Bool() && img.Get("naturalWidth").Int() > 0 {
		app.ui.ctx.Call("drawImage", img, x, y, tileSize, tileSize)
	} else {
		app.ui.ctx.Set("fillStyle", "white")
		if p.Color() == chess.Black {
			app.ui.ctx.Set("fillStyle", "#aaa")
		}
		app.ui.ctx.Set("font", "bold 30px Arial")
		app.ui.ctx.Call("fillText", pStr, x+tileSize/4, y+tileSize/1.5)
	}
}
