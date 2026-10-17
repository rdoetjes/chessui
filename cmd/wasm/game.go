//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"

	"github.com/notnil/chess"
)

// getViewGame returns a game state representing the move currently being viewed.
type GameState struct {
	game             *chess.Game
	selectedPos      chess.Square
	lastMove         *chess.Move
	isGameOver       bool
	isSetupMode      bool
	setupSquare      chess.Square
	setupMap         map[chess.Square]chess.Piece
	currentViewIndex int // -1 means viewing the latest move
	moveAnnotations  []string
	initialFEN       string
	cachedViewGame   *chess.Game
	cachedViewIndex  int  // -2 means invalid
	isWhite          bool // Board orientation
}

func getViewGame() *chess.Game {
	if app.game.currentViewIndex == -1 {
		return app.game.game
	}
	// Check cache
	// Use -1 for "Initial Position" logic in cache
	lookupIndex := app.game.currentViewIndex
	if lookupIndex == -2 {
		lookupIndex = -1
	}

	if app.game.cachedViewIndex == lookupIndex && app.game.cachedViewGame != nil {
		return app.game.cachedViewGame
	}

	var tempGame *chess.Game
	if app.game.initialFEN != "" {
		f, _ := chess.FEN(app.game.initialFEN)
		tempGame = chess.NewGame(f)
	} else {
		tempGame = chess.NewGame()
	}
	moves := app.game.game.Moves()
	for i := 0; i <= lookupIndex && i < len(moves); i++ {
		tempGame.Move(moves[i])
	}

	app.game.cachedViewGame = tempGame
	app.game.cachedViewIndex = lookupIndex
	return tempGame
}

// executeMove is the central point for ALL movements.
func executeMove(moveStr string, relay bool) {
	// 1. Handle Branching from History
	if app.game.currentViewIndex != -1 {
		truncIdx := app.game.currentViewIndex
		if truncIdx == -2 {
			truncIdx = -1
		}
		truncateGameAt(truncIdx)
		app.game.currentViewIndex = -1 // Back to live
	}

	// 2. Find the move in the list of legal moves for the CURRENT game
	var target *chess.Move
	for _, m := range app.game.game.ValidMoves() {
		// Use coordinate comparison (UCI standard)
		if m.String() == moveStr {
			target = m
			break
		}
	}

	// 2. If move is valid, apply it
	if target != nil {
		// If we are making a move, the game is no longer over (unless the new move results in mate)
		app.game.isGameOver = false
		doc := js.Global().Get("document")
		doc.Call("getElementById", "checkmate-alert").Get("classList").Call("add", "hidden")

		// Record state BEFORE move
		app.engine.lastPositionEval = app.ui.evalScore
		app.engine.lastMoveSide = app.game.game.Position().Turn()

		// Show what the engine thought was best for the position we JUST left
		annotation := ""
		if app.engine.lastHumanSuggestion != "" && relay {
			text := "Best move was: " + app.engine.lastHumanSuggestion
			if moveStr == app.engine.lastHumanSuggestion {
				text += " 👍"
				annotation = "🟢"
			}
			showSuggestion(text)
		}
		app.game.moveAnnotations = append(app.game.moveAnnotations, annotation)
		app.engine.lastHumanSuggestion = "" // Clear for next turn

		// Trigger animation
		app.anim.move = target
		app.anim.piece = app.game.game.Position().Board().Piece(target.S1())
		app.anim.progress = 0.0
		app.anim.startTime = js.Global().Get("performance").Call("now").Float()

		if err := app.game.game.Move(target); err == nil {
			app.game.lastMove = target
			updateMoveList()
			checkGameState()
			playSound(target)

			// Clear selection
			app.game.selectedPos = chess.NoSquare

			// 3. Multiplayer Relay
			if relay && app.multi.isActive {
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
	if app.game.isGameOver {
		return
	}
	sq := chess.Square(row*8 + col)
	fmt.Printf("WASM: Board Click: %s (col=%d, row=%d)\n", sq.String(), col, row)

	if app.game.isSetupMode {
		app.game.setupSquare = sq
		js.Global().Get("document").Call("getElementById", "piece-picker").Get("classList").Call("remove", "hidden")
		return
	}

	viewGame := getViewGame()
	pos := viewGame.Position()
	turn := pos.Turn()

	if app.game.selectedPos == chess.NoSquare {
		p := pos.Board().Piece(sq)
		// Allow selecting pieces even if it's not the player's turn (for visualization)
		if p != chess.NoPiece && p.Color() == turn {
			app.game.selectedPos = sq
		}
	} else {
		// Only allow executing moves if it's the player's turn
		if turn != app.multi.playerColor {
			app.game.selectedPos = chess.NoSquare
			return
		}

		var targetMoves []*chess.Move
		for _, m := range viewGame.ValidMoves() {
			if m.S1() == app.game.selectedPos && m.S2() == sq {
				targetMoves = append(targetMoves, m)
			}
		}

		if len(targetMoves) == 1 {
			executeMove(targetMoves[0].String(), true)
		} else if len(targetMoves) > 1 {
			// Promotion detected
			app.ui.isPromotionMode = true
			app.ui.promoSourceSquare = app.game.selectedPos
			app.ui.promoTargetSquare = sq
			showPromotionPicker(turn)
		} else {
			p := pos.Board().Piece(sq)
			if p != chess.NoPiece && p.Color() == turn {
				app.game.selectedPos = sq
			} else {
				app.game.selectedPos = chess.NoSquare
			}
		}
	}
}

func checkGameState() {
	if app.game.game.Outcome() != chess.NoOutcome {
		app.game.isGameOver = true
		alert := js.Global().Get("document").Call("getElementById", "checkmate-alert")
		alert.Call("querySelector", "#winner-text").Set("innerText", "Outcome: "+app.game.game.Outcome().String())
		alert.Get("classList").Call("remove", "hidden")
	}
}

func truncateGameAt(index int) {
	allMoves := app.game.game.Moves()
	if index < -1 || index >= len(allMoves) {
		return
	}

	newMoves := allMoves[:index+1]

	// Truncate annotations to match
	if len(app.game.moveAnnotations) > len(newMoves) {
		app.game.moveAnnotations = app.game.moveAnnotations[:len(newMoves)]
	}

	// Reset game and re-apply moves
	var newGame *chess.Game
	if app.game.initialFEN != "" {
		f, _ := chess.FEN(app.game.initialFEN)
		newGame = chess.NewGame(f)
	} else {
		newGame = chess.NewGame()
	}

	for _, m := range newMoves {
		newGame.Move(m)
	}
	app.game.game = newGame

	// Invalidate view cache
	app.game.cachedViewIndex = -2
	app.game.cachedViewGame = nil
}

func playSound(m *chess.Move) {
	doc := js.Global().Get("document")
	var audio js.Value

	if m.HasTag(chess.Check) {
		audio = doc.Call("getElementById", "snd-check")
	} else if m.HasTag(chess.Capture) || m.HasTag(chess.EnPassant) {
		audio = doc.Call("getElementById", "snd-capture")
	} else {
		audio = doc.Call("getElementById", "snd-move")
	}

	if !audio.IsUndefined() && !audio.IsNull() {
		// Reset current time to allow playing overlapping sounds
		audio.Set("currentTime", 0)
		promise := audio.Call("play")
		if !promise.IsUndefined() {
			promise.Call("catch", js.FuncOf(func(this js.Value, args []js.Value) any {
				// Browsers block audio until first interaction
				return nil
			}))
		}
	}
}
