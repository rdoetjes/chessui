//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"
)

var currentMoveIndex int = 0

// handleBotTurn is the ONLY place that tells the engine to start thinking.
func handleBotTurn() {
	if isSetupMode || isGameOver {
		return
	}

	currentMoveIndex++
	myMoveIndex := currentMoveIndex

	// 1. Force the engine to stop any previous analysis
	sendUCI("stop")

	// 2. Command the engine to find a move/suggestion
	turn := game.Position().Turn()
	currentFen := game.Position().String()

	// Set engine strength
	eloStr := js.Global().Get("document").Call("getElementById", "elo-select").Get("value").String()

	// If Elo is 2850 (Max), disable limit strength to allow full configured depth
	if eloStr == "2850" {
		sendUCI("setoption name UCI_LimitStrength value false")
	} else {
		sendUCI("setoption name UCI_LimitStrength value true")
		sendUCI("setoption name UCI_Elo value " + eloStr)
	}

	sendUCI("position fen " + currentFen)
	sendUCI("isready")

	// In multiplayer, we always just analyze (no matter whose turn)
	// In single player, we analyze on our turn and think on bot's turn
	if isMultiplayer {
		fmt.Printf("WASM: [ID:%d] Multiplayer Analysis. FEN: %s\n", myMoveIndex, currentFen)
		sendUCI(fmt.Sprintf("go depth %d", engineDepth))
	} else if turn != playerColor {
		fmt.Printf("WASM: [ID:%d] BOT Turn. Thinking...\n", myMoveIndex)
		sendUCI(fmt.Sprintf("go depth %d", engineDepth))
	} else {
		sendUCI(fmt.Sprintf("go depth %d", engineDepth))
	}
}

// processBotResponse handles the 'bestmove' string from Stockfish.
func processBotResponse(moveStr string) {
	if isGameOver || moveStr == "(none)" {
		return
	}

	// MULTIPLAYER: Only use bestmove for suggestions, NEVER execute
	if isMultiplayer {
		lastHumanSuggestion = moveStr
		return
	}

	// SINGLE PLAYER:
	// If it's currently the human's turn, this is a background suggestion.
	if game.Position().Turn() == playerColor {
		lastHumanSuggestion = moveStr
		return
	}

	// If it's the bot's turn, this is the move it wants to play.
	fmt.Printf("WASM: Executing BOT move: %s\n", moveStr)
	executeMove(moveStr, false)
}

func showSuggestion(text string) {
	doc := js.Global().Get("document")
	el := doc.Call("getElementById", "engine-suggestion")
	if !el.IsNull() {
		el.Set("innerText", text)
	}
}
