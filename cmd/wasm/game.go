//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"

	"github.com/notnil/chess"
)

// executeMove is the central point for ALL movements.
func executeMove(moveStr string, relay bool) {
	// 1. Find the move in the list of legal moves
	var target *chess.Move
	for _, m := range game.ValidMoves() {
		// Use coordinate comparison (UCI standard)
		if m.String() == moveStr {
			target = m
			break
		}
	}

	// 2. If move is valid, apply it
	if target != nil {
		if isGameOver {
			return
		}
		// Show what the engine thought was best for the position we JUST left
		if lastHumanSuggestion != "" {
			text := "Best move was: " + lastHumanSuggestion
			if moveStr == lastHumanSuggestion {
				text += " 👍"
			}
			showSuggestion(text)
		}
		lastHumanSuggestion = "" // Clear for next turn

		// Trigger animation
		animMove = target
		animPiece = game.Position().Board().Piece(target.S1())
		animProgress = 0.0
		animStartTime = js.Global().Get("performance").Call("now").Float()

		if err := game.Move(target); err == nil {
			lastMove = target
			updateMoveList()
			checkGameState()

			// Clear selection
			selectedPos = chess.NoSquare

			// 3. Multiplayer Relay
			if relay && isMultiplayer {
				relayMoveToOpponent(moveStr)
			}

			// 4. Kick off the engine for the next turn
			handleBotTurn()
			updateCapturedPieces()
		} else {
			fmt.Printf("WASM: Move error: %v\n", err)
		}
	} else {
		fmt.Printf("WASM: Illegal move received from engine: %s\n", moveStr)
	}
}

func handleBoardClick(col, row int) {
	if isGameOver {
		return
	}
	sq := chess.Square(row*8 + col)
	fmt.Printf("WASM: Board Click: %s (col=%d, row=%d)\n", sq.String(), col, row)

	if isSetupMode {
		setupSquare = sq
		js.Global().Get("document").Call("getElementById", "piece-picker").Get("classList").Call("remove", "hidden")
		return
	}

	turn := game.Position().Turn()
	if turn != playerColor {
		return
	}

	if selectedPos == chess.NoSquare {
		p := game.Position().Board().Piece(sq)
		if p != chess.NoPiece && p.Color() == turn {
			selectedPos = sq
		}
	} else {
		var targetStr string
		for _, m := range game.ValidMoves() {
			if m.S1() == selectedPos && m.S2() == sq {
				targetStr = m.String()
				break
			}
		}

		if targetStr != "" {
			executeMove(targetStr, true) // true = relay if multiplayer
		} else {
			p := game.Position().Board().Piece(sq)
			if p != chess.NoPiece && p.Color() == turn {
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
