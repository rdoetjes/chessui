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
	isStarted        bool // Whether the game has actually started
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
	handleHistoryBranching()

	target := findMove(moveStr)
	if target == nil {
		fmt.Printf("WASM: Illegal move received: %s\n", moveStr)
		return
	}

	applyMove(target, moveStr, relay)
}

func handleHistoryBranching() {
	if app.game.currentViewIndex == -1 {
		return
	}

	truncIdx := app.game.currentViewIndex
	if truncIdx == -2 {
		truncIdx = -1
	}
	truncateGameAt(truncIdx)
	app.game.currentViewIndex = -1 // Back to live
}

func findMove(moveStr string) *chess.Move {
	for _, m := range app.game.game.ValidMoves() {
		if m.String() == moveStr {
			return m
		}
	}
	return nil
}

func applyMove(target *chess.Move, moveStr string, relay bool) {
	app.game.isGameOver = false

	// Record state BEFORE move
	app.engine.lastPositionEval = app.ui.evalScore
	app.engine.lastMoveSide = app.game.game.Position().Turn()

	updateMoveAnnotations(moveStr, relay)
	triggerMoveAnimation(target)

	if err := app.game.game.Move(target); err == nil {
		postMoveCleanup(target, moveStr, relay)
	} else {
		fmt.Printf("WASM: Move error: %v\n", err)
	}
}

func updateMoveAnnotations(moveStr string, relay bool) {
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
}

func triggerMoveAnimation(target *chess.Move) {
	app.anim.move = target
	app.anim.piece = app.game.game.Position().Board().Piece(target.S1())
	app.anim.progress = 0.0
	app.anim.startTime = js.Global().Get("performance").Call("now").Float()
}

func postMoveCleanup(target *chess.Move, moveStr string, relay bool) {
	app.game.lastMove = target
	updateMoveList()
	checkGameState()
	playSound(target)

	// Clear selection
	app.game.selectedPos = chess.NoSquare

	// Multiplayer Relay
	if relay && app.multi.isActive {
		relayMoveToOpponent(moveStr)
	}

	// Kick off the engine for the next turn
	handleBotTurn()
	updateCapturedPieces()
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
		// Only allow executing moves if it's the player's turn (in live mode)
		if app.game.currentViewIndex == -1 && turn != app.multi.playerColor {
			app.game.selectedPos = chess.NoSquare
			return
		}

		// When playing from history, we automatically adopt the side that is moving
		if app.game.currentViewIndex != -1 {
			app.multi.playerColor = turn
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
	// Only set game over if the position is actually terminal.
	// This prevents PGN [Result] tags from ending the game prematurely
	// when we are trying to play on from an imported position.
	status := app.game.game.Position().Status()
	if status != chess.NoMethod {
		app.game.isGameOver = true
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
