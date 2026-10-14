//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/notnil/chess"
)

func handleBoardClick(col, row int) {
	sq := chess.Square(row*8 + col)
	if isSetupMode {
		setupSquare = sq
		js.Global().Get("document").Call("getElementById", "piece-picker").Get("classList").Call("remove", "hidden")
		return
	}

	// Turn check
	turn := game.Position().Turn()

	// Enforcement for Multiplayer: Can only move pieces of your color
	if isMultiplayer && turn != playerColor {
		return
	}

	if selectedPos == chess.NoSquare {
		p := game.Position().Board().Piece(sq)
		if p != chess.NoPiece && p.Color() == turn {
			selectedPos = sq
		}
	} else {
		var targetMove *chess.Move
		for _, m := range game.ValidMoves() {
			if m.S1() == selectedPos && m.S2() == sq {
				targetMove = m
				break
			}
		}

		if targetMove != nil {
			game.Move(targetMove)
			lastMove = targetMove
			selectedPos = chess.NoSquare
			updateMoveList()
			sendMoveToStockfish()
			checkGameState()
		} else {
			p := game.Position().Board().Piece(sq)
			if p != chess.NoPiece && p.Color() == game.Position().Turn() {
				selectedPos = sq
			} else {
				selectedPos = chess.NoSquare
			}
		}
	}
}

func checkGameState() {
	if game.Outcome() != chess.NoOutcome {
		isGameOver = true
		alert := js.Global().Get("document").Call("getElementById", "checkmate-alert")
		alert.Call("querySelector", "#winner-text").Set("innerText", "Outcome: "+game.Outcome().String())
		alert.Get("classList").Call("remove", "hidden")
	}
}
