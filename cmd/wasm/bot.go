//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"
)

var currentMoveIndex int = 0

// handleBotTurn is the ONLY place that tells the engine to start thinking.
func handleBotTurn() {
	if isMultiplayer || isSetupMode || isGameOver {
		return
	}

	currentMoveIndex++
	myMoveIndex := currentMoveIndex

	// 1. Force the engine to stop any previous analysis
	sendUCI("stop")

	// 2. Command the bot to find a move
	turn := game.Position().Turn()
	currentFen := game.Position().String()

	// Set engine strength
	elo := js.Global().Get("document").Call("getElementById", "elo-select").Get("value").String()
	sendUCI("setoption name UCI_LimitStrength value true")
	sendUCI("setoption name UCI_Elo value " + elo)

	sendUCI("position fen " + currentFen)
	sendUCI("isready")

	if turn != playerColor {
		fmt.Printf("WASM: [ID:%d] BOT Turn. Thinking...\n", myMoveIndex)
		sendUCI(fmt.Sprintf("go depth %d", engineDepth))
	} else {
		// Just run a search to update the eval bar during human turn
		sendUCI(fmt.Sprintf("go depth %d", engineDepth))
	}
}

// processBotResponse handles the 'bestmove' string from Stockfish.
func processBotResponse(moveStr string) {
	if isMultiplayer || isGameOver || moveStr == "(none)" {
		return
	}

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
