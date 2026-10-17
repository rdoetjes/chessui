//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"syscall/js"

	"github.com/notnil/chess"
	"github.com/rdoetjes/chess-ui/internal/uci"
)

type EngineState struct {
	threads             int
	lastHumanSuggestion string
	lastPositionEval    float64
	lastMoveSide        chess.Color
}

func setupWebSocket() {
	loc := js.Global().Get("location")
	host := loc.Get("host").String()
	roomID := "bot-" + generateUUID()
	app.multi.isActive = false

	params := js.Global().Get("URLSearchParams").New(loc.Get("search"))
	roomParam := params.Call("get", "room")

	// FIX: Robust check for null/undefined/empty room parameter
	app.multi.isActive = false
	app.multi.multiplayerColor = "white"
	if !roomParam.IsNull() && !roomParam.IsUndefined() {
		val := roomParam.String()
		if val != "" && val != "null" && val != "<null>" {
			roomID = val
			app.multi.isActive = true
			app.multi.multiplayerColor = "auto"
			fmt.Printf("WASM: Multiplayer Mode Active (Room: %s)\n", roomID)
		}
	}

	if !app.multi.isActive {
		fmt.Println("WASM: Single Player Mode (vs BOT)")
		app.game.isStarted = false
	} else {
		app.game.isStarted = true
	}

	connect(host, roomID, loc.Get("protocol").String() == "https:")
}

func connect(host, roomID string, isSecure bool) {
	wsURL := "ws://" + host + "/ws?room=" + roomID
	if isSecure {
		wsURL = "wss://" + host + "/ws?room=" + roomID
	}
	app.multi.ws = js.Global().Get("WebSocket").New(wsURL)

	app.multi.ws.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println("WASM: WebSocket connected")
		doc := js.Global().Get("document")
		statusLine := doc.Call("getElementById", "wasm-status")
		if !statusLine.IsNull() {
			if app.multi.isActive {
				statusLine.Set("innerText", "CHESS UI - MULTIPLAYER")
			} else {
				statusLine.Set("innerText", "CHESS UI - BOT ACTIVE")
			}
			statusLine.Get("style").Set("color", "#ffffff")
		}

		sendUCI("uci")
		sendUCI(fmt.Sprintf("setoption name Threads value %d", app.engine.threads))
		sendUCI("isready")
		sendUCI("ucinewgame")
		sendUCI("isready")
		if app.multi.isActive {
			if app.multi.multiplayerColor == "auto" {
				sendUCI("request_color")
			} else {
				sendUCI("player_color " + app.multi.multiplayerColor)
			}
		} else {
			app.multi.playerColor = chess.White
		}
		handleBotTurn()
		return nil
	}))

	app.multi.ws.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println("WASM: WebSocket Error")
		return nil
	}))

	app.multi.ws.Set("onclose", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println("WASM: WebSocket Closed")
		return nil
	}))

	app.multi.ws.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		msg := args[0].Get("data").String()

		// Handle JSON config messages (more robust check)
		if strings.HasPrefix(msg, "{") && strings.Contains(msg, "\"type\":\"config\"") {
			var cfg struct {
				Type    string `json:"type"`
				Threads int    `json:"threads"`
			}
			if err := json.Unmarshal([]byte(msg), &cfg); err == nil && cfg.Type == "config" {
				app.engine.threads = cfg.Threads
				fmt.Printf("WASM: Config received - Threads: %d\n", app.engine.threads)
				sendUCI(fmt.Sprintf("setoption name Threads value %d", app.engine.threads))
			}
			return nil
		}

		if handleMultiplayerMessage(msg) {
			return nil
		}
		processUCIResponse(msg)
		return nil
	}))
}

func processUCIResponse(msg string) {
	info := uci.ParseLine(msg)
	doc := js.Global().Get("document")
	if info.HasScore {
		infoPanel := doc.Call("getElementById", "engine-info")
		scoreStr := fmt.Sprintf("%.1f", float64(info.Score.CP)/100.0)
		if info.Score.IsMate {
			scoreStr = fmt.Sprintf("M%d", info.Score.Mate)
		}
		infoPanel.Set("innerText", fmt.Sprintf("Depth %d | Score %s", info.Depth, scoreStr))
		sVal := float64(info.Score.CP) / 100.0
		if info.Score.IsMate {
			sVal = 10.0
			if info.Score.Mate < 0 {
				sVal = -10.0
			}
		}
		if app.game.game.Position().Turn() == chess.Black {
			sVal = -sVal
		}
		app.ui.evalScore = sVal
		updateEvalBar()

		// Check for move quality on the move just made
		// Only rate if we have reached a reasonable depth to avoid noise
		if info.Depth >= 8 && len(app.game.moveAnnotations) > 0 && app.engine.lastMoveSide == app.multi.playerColor {
			lastIdx := len(app.game.moveAnnotations) - 1
			var diff float64
			if app.engine.lastMoveSide == chess.White {
				diff = app.ui.evalScore - app.engine.lastPositionEval
			} else {
				diff = app.engine.lastPositionEval - app.ui.evalScore
			}

			rating := colorMoveRating(diff)
			// Never downgrade an 'Excellent' (Best) move
			// Only apply rating if it's "worse" than what's already there (or if nothing is there)
			if rating != "" && app.game.moveAnnotations[lastIdx] != "🟢" {
				app.game.moveAnnotations[lastIdx] = rating
				updateMoveList()
			}
		}
	}
	if info.BestMove != "" {
		fmt.Printf("WASM: BestMove received: %s\n", info.BestMove)
		if !app.game.isGameOver {
			processBotResponse(info.BestMove)
		}
	}
}

func reconnectSession(roomID string, color string) {
	if app.multi.ws.Get("readyState").Int() == 1 {
		app.multi.ws.Call("close")
	}
	if roomID == "bot-play" {
		roomID = "bot-" + generateUUID()
		app.multi.isActive = false
	} else {
		app.multi.isActive = true
	}
	doc := js.Global().Get("document")
	if app.multi.isActive {
		doc.Call("getElementById", "multiplayer-buttons").Get("classList").Call("add", "hidden")
		doc.Call("getElementById", "take-back").Get("classList").Call("add", "hidden")
	} else {
		doc.Call("getElementById", "take-back").Get("classList").Call("remove", "hidden")
		doc.Call("getElementById", "multiplayer-buttons").Get("classList").Call("remove", "hidden")
	}
	if color != "auto" {
		app.multi.playerColor = playerColorMap[color]
		app.multi.multiplayerColor = color
	} else {
		app.multi.multiplayerColor = "auto"
	}

	app.game.game = chess.NewGame()
	app.game.lastMove = nil
	app.game.moveAnnotations = nil
	app.game.currentViewIndex = -1
	app.game.cachedViewIndex = -2
	app.game.initialFEN = ""
	app.game.isStarted = true
	updateMoveList()
	updateCapturedPieces()

	loc := js.Global().Get("location")
	connect(loc.Get("host").String(), roomID, loc.Get("protocol").String() == "https:")
}

func sendUCI(msg string) {
	if app.multi.ws.Get("readyState").Int() == 1 {
		app.multi.ws.Call("send", msg)
	}
}

func colorMoveRating(diff float64) string {
	if diff < -2.5 {
		return "🔴"
	}
	if diff < -1.2 {
		return "🟡"
	}
	if diff < -0.6 {
		return "🟠"
	}
	if diff > 0.2 {
		return "🟢"
	}
	return ""
}
