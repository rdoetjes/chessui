//go:build js && wasm

package main

import (
	"fmt"
	"strconv"
	"syscall/js"
)

type EngineSettings struct {
	LimitStrength bool
	Elo           int
	SkillLevel    int
	Depth         int
	MoveTime      int
}

func getEngineSettings(elo int) EngineSettings {
	switch elo {
	case 100:
		return EngineSettings{LimitStrength: true, Elo: 1350, SkillLevel: 0, Depth: 1, MoveTime: 20}
	case 300:
		return EngineSettings{LimitStrength: true, Elo: 1350, SkillLevel: 1, Depth: 2, MoveTime: 30}
	case 500:
		return EngineSettings{LimitStrength: true, Elo: 1350, SkillLevel: 2, Depth: 2, MoveTime: 50}
	case 700:
		return EngineSettings{LimitStrength: true, Elo: 700, SkillLevel: -1, Depth: 3, MoveTime: 80}
	case 900:
		return EngineSettings{LimitStrength: true, Elo: 900, SkillLevel: -1, Depth: 4, MoveTime: 100}
	case 1100:
		return EngineSettings{LimitStrength: true, Elo: 1100, SkillLevel: -1, Depth: 5, MoveTime: 150}
	case 1300:
		return EngineSettings{LimitStrength: true, Elo: 1300, SkillLevel: -1, Depth: 6, MoveTime: 200}
	case 1500:
		return EngineSettings{LimitStrength: true, Elo: 1500, SkillLevel: -1, Depth: 8, MoveTime: 300}
	case 1700:
		return EngineSettings{LimitStrength: true, Elo: 1700, SkillLevel: -1, Depth: 10, MoveTime: 400}
	case 1900:
		return EngineSettings{LimitStrength: true, Elo: 1900, SkillLevel: -1, Depth: 12, MoveTime: 500}
	case 2100:
		return EngineSettings{LimitStrength: false, Depth: 14, MoveTime: 800}
	case 2300:
		return EngineSettings{LimitStrength: false, Depth: 16, MoveTime: 1000}
	case 2500:
		return EngineSettings{LimitStrength: false, Depth: 18, MoveTime: 1500}
	case 2700:
		return EngineSettings{LimitStrength: false, Depth: 20, MoveTime: 2000}
	default:
		return EngineSettings{LimitStrength: true, Elo: elo, SkillLevel: -1, Depth: 5, MoveTime: 150}
	}
}

var currentMoveIndex int = 0

// handleBotTurn is the ONLY place that tells the engine to start thinking.
func handleBotTurn() {
	if app.game.isSetupMode || app.game.isGameOver {
		return
	}

	// If we are viewing history, the bot shouldn't move.
	if app.game.currentViewIndex != -1 {
		return
	}

	currentMoveIndex++
	myMoveIndex := currentMoveIndex

	// 1. Force the engine to stop any previous analysis
	sendUCI("stop")

	viewGame := getViewGame()

	// 2. Command the engine to find a move/suggestion
	turn := viewGame.Position().Turn()
	currentFen := viewGame.Position().String()

	// Set engine strength
	eloStr := js.Global().Get("document").Call("getElementById", "elo-select").Get("value").String()
	eloVal, _ := strconv.Atoi(eloStr)
	settings := getEngineSettings(eloVal)

	if !settings.LimitStrength {
		sendUCI("setoption name UCI_LimitStrength value false")
	} else {
		sendUCI("setoption name UCI_LimitStrength value true")
		sendUCI(fmt.Sprintf("setoption name UCI_Elo value %d", settings.Elo))
		if settings.SkillLevel >= 0 {
			sendUCI(fmt.Sprintf("setoption name Skill Level value %d", settings.SkillLevel))
		} else {
			// Reset skill level to max if not using it
			sendUCI("setoption name Skill Level value 20")
		}
	}

	sendUCI("position fen " + currentFen)
	sendUCI("isready")

	goCommand := fmt.Sprintf("go depth %d movetime %d", settings.Depth, settings.MoveTime)

	// In multiplayer, we always just analyze (no matter whose turn)
	// In single player, we analyze on our turn and think on bot's turn
	if app.multi.isActive {
		fmt.Printf("WASM: [ID:%d] Multiplayer Analysis. FEN: %s\n", myMoveIndex, currentFen)
		sendUCI(goCommand)
	} else if turn != app.multi.playerColor {
		fmt.Printf("WASM: [ID:%d] BOT Turn. Thinking...\n", myMoveIndex)
		sendUCI(goCommand)
	} else {
		sendUCI(goCommand)
	}
}

// processBotResponse handles the 'bestmove' string from Stockfish.
func processBotResponse(moveStr string) {
	if app.game.isGameOver || moveStr == "(none)" {
		return
	}

	// MULTIPLAYER: Only use bestmove for suggestions, NEVER execute
	if app.multi.isActive {
		app.engine.lastHumanSuggestion = moveStr
		return
	}

	// SINGLE PLAYER:
	// If we are viewing history, don't execute bot moves
	if app.game.currentViewIndex != -1 {
		app.engine.lastHumanSuggestion = moveStr
		return
	}

	// If it's currently the human's turn, this is a background suggestion.
	if app.game.game.Position().Turn() == app.multi.playerColor {
		app.engine.lastHumanSuggestion = moveStr
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
