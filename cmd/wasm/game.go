//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"

	"github.com/notnil/chess"
)

// getViewGame returns a game state representing the move currently being viewed.
func getViewGame() *chess.Game {
	if currentViewIndex == -1 {
		return game
	}
	// Check cache
	// Use -1 for "Initial Position" logic in cache
	lookupIndex := currentViewIndex
	if lookupIndex == -2 {
		lookupIndex = -1
	}

	if cachedViewIndex == lookupIndex && cachedViewGame != nil {
		return cachedViewGame
	}

	var tempGame *chess.Game
	if initialFEN != "" {
		f, _ := chess.FEN(initialFEN)
		tempGame = chess.NewGame(f)
	} else {
		tempGame = chess.NewGame()
	}
	moves := game.Moves()
	for i := 0; i <= lookupIndex && i < len(moves); i++ {
		tempGame.Move(moves[i])
	}

	cachedViewGame = tempGame
	cachedViewIndex = lookupIndex
	return tempGame
}

// executeMove is the central point for ALL movements.
func executeMove(moveStr string, relay bool) {
	// 1. Handle Branching from History
	if currentViewIndex != -1 {
		truncIdx := currentViewIndex
		if truncIdx == -2 {
			truncIdx = -1
		}
		truncateGameAt(truncIdx)
		currentViewIndex = -1 // Back to live
	}

	// 2. Find the move in the list of legal moves for the CURRENT game
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
		// If we are making a move, the game is no longer over (unless the new move results in mate)
		isGameOver = false
		doc := js.Global().Get("document")
		doc.Call("getElementById", "checkmate-alert").Get("classList").Call("add", "hidden")

		// Record state BEFORE move
		lastPositionEval = evalScore
		lastMoveSide = game.Position().Turn()

		// Show what the engine thought was best for the position we JUST left
		annotation := ""
		if lastHumanSuggestion != "" && relay {
			text := "Best move was: " + lastHumanSuggestion
			if moveStr == lastHumanSuggestion {
				text += " 👍"
				annotation = "🟢"
			}
			showSuggestion(text)
		}
		moveAnnotations = append(moveAnnotations, annotation)
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
			playSound(target)

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

	viewGame := getViewGame()
	pos := viewGame.Position()
	turn := pos.Turn()

	if selectedPos == chess.NoSquare {
		p := pos.Board().Piece(sq)
		// Allow selecting pieces even if it's not the player's turn (for visualization)
		if p != chess.NoPiece && p.Color() == turn {
			selectedPos = sq
		}
	} else {
		// Only allow executing moves if it's the player's turn
		if turn != playerColor {
			selectedPos = chess.NoSquare
			return
		}

		var targetMoves []*chess.Move
		for _, m := range viewGame.ValidMoves() {
			if m.S1() == selectedPos && m.S2() == sq {
				targetMoves = append(targetMoves, m)
			}
		}

		if len(targetMoves) == 1 {
			executeMove(targetMoves[0].String(), true)
		} else if len(targetMoves) > 1 {
			// Promotion detected
			isPromotionMode = true
			promoSourceSquare = selectedPos
			promoTargetSquare = sq
			showPromotionPicker(turn)
		} else {
			p := pos.Board().Piece(sq)
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

func truncateGameAt(index int) {
	allMoves := game.Moves()
	if index < -1 || index >= len(allMoves) {
		return
	}

	newMoves := allMoves[:index+1]

	// Truncate annotations to match
	if len(moveAnnotations) > len(newMoves) {
		moveAnnotations = moveAnnotations[:len(newMoves)]
	}

	// Reset game and re-apply moves
	var newGame *chess.Game
	if initialFEN != "" {
		f, _ := chess.FEN(initialFEN)
		newGame = chess.NewGame(f)
	} else {
		newGame = chess.NewGame()
	}

	for _, m := range newMoves {
		newGame.Move(m)
	}
	game = newGame

	// Invalidate view cache
	cachedViewIndex = -2
	cachedViewGame = nil
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
