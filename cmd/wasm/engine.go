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

func setupWebSocket() {
	loc := js.Global().Get("location")
	host := loc.Get("host").String()
	roomID := "bot-" + generateUUID()
	isMultiplayer = false

	params := js.Global().Get("URLSearchParams").New(loc.Get("search"))
	roomParam := params.Call("get", "room")

	// FIX: Robust check for null/undefined/empty room parameter
	isMultiplayer = false
	multiplayerColor = "white"
	if !roomParam.IsNull() && !roomParam.IsUndefined() {
		val := roomParam.String()
		if val != "" && val != "null" && val != "<null>" {
			roomID = val
			isMultiplayer = true
			multiplayerColor = "auto"
			fmt.Printf("WASM: Multiplayer Mode Active (Room: %s)\n", roomID)
		}
	}

	if !isMultiplayer {
		fmt.Println("WASM: Single Player Mode (vs BOT)")
	}

	connect(host, roomID, loc.Get("protocol").String() == "https:")
}

func connect(host, roomID string, isSecure bool) {
	wsURL := "ws://" + host + "/ws?room=" + roomID
	if isSecure {
		wsURL = "wss://" + host + "/ws?room=" + roomID
	}
	ws = js.Global().Get("WebSocket").New(wsURL)

	ws.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println("WASM: WebSocket connected")
		doc := js.Global().Get("document")
		statusLine := doc.Call("getElementById", "wasm-status")
		if !statusLine.IsNull() {
			if isMultiplayer {
				statusLine.Set("innerText", "CHESS UI - MULTIPLAYER")
			} else {
				statusLine.Set("innerText", "CHESS UI - BOT ACTIVE")
			}
			statusLine.Get("style").Set("color", "#ffffff")
		}

		sendUCI("uci")
		sendUCI(fmt.Sprintf("setoption name Threads value %d", engineThreads))
		sendUCI("isready")
		sendUCI("ucinewgame")
		sendUCI("isready")
		if isMultiplayer {
			if multiplayerColor == "auto" {
				sendUCI("request_color")
			} else {
				sendUCI("player_color " + multiplayerColor)
			}
		} else {
			playerColor = chess.White
		}
		handleBotTurn()
		return nil
	}))

	ws.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println("WASM: WebSocket Error")
		return nil
	}))

	ws.Set("onclose", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println("WASM: WebSocket Closed")
		return nil
	}))

	ws.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		msg := args[0].Get("data").String()

		// Handle JSON config messages (more robust check)
		if strings.HasPrefix(msg, "{") && strings.Contains(msg, "\"type\":\"config\"") {
			var cfg struct {
				Type    string `json:"type"`
				Depth   int    `json:"depth"`
				Threads int    `json:"threads"`
			}
			if err := json.Unmarshal([]byte(msg), &cfg); err == nil && cfg.Type == "config" {
				engineDepth = cfg.Depth
				engineThreads = cfg.Threads
				fmt.Printf("WASM: Config received - Depth: %d, Threads: %d\n", engineDepth, engineThreads)
				sendUCI(fmt.Sprintf("setoption name Threads value %d", engineThreads))
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
		if game.Position().Turn() == chess.Black {
			sVal = -sVal
		}
		evalScore = sVal
		updateEvalBar()

		// Check for blunder on the move just made
		if len(moveAnnotations) > 0 && lastMoveSide == playerColor {
			lastIdx := len(moveAnnotations) - 1
			// Only check if not already marked (e.g. as 🟢)
			if moveAnnotations[lastIdx] == "" {
				var diff float64
				if lastMoveSide == chess.White {
					diff = evalScore - lastPositionEval
				} else {
					diff = lastPositionEval - evalScore
				}

				if diff < -2.0 {
					moveAnnotations[lastIdx] = "🔴"
					updateMoveList()
				}
			}
		}
	}
	if info.BestMove != "" {
		fmt.Printf("WASM: BestMove received: %s\n", info.BestMove)
		if !isGameOver {
			processBotResponse(info.BestMove)
		}
	}
}

func reconnectSession(roomID string, color string) {
	if ws.Get("readyState").Int() == 1 {
		ws.Call("close")
	}
	if roomID == "bot-play" {
		roomID = "bot-" + generateUUID()
		isMultiplayer = false
	} else {
		isMultiplayer = true
	}
	doc := js.Global().Get("document")
	if isMultiplayer {
		doc.Call("getElementById", "multiplayer-buttons").Get("classList").Call("add", "hidden")
		doc.Call("getElementById", "take-back").Get("classList").Call("add", "hidden")
	} else {
		doc.Call("getElementById", "take-back").Get("classList").Call("remove", "hidden")
		doc.Call("getElementById", "multiplayer-buttons").Get("classList").Call("remove", "hidden")
	}
	if color != "auto" {
		playerColor = playerColorMap[color]
		multiplayerColor = color
	} else {
		multiplayerColor = "auto"
	}

	game = chess.NewGame()
	lastMove = nil
	moveAnnotations = nil
	initialFEN = ""
	updateMoveList()
	updateCapturedPieces()

	loc := js.Global().Get("location")
	connect(loc.Get("host").String(), roomID, loc.Get("protocol").String() == "https:")
}

func sendUCI(msg string) {
	if ws.Get("readyState").Int() == 1 {
		ws.Call("send", msg)
	}
}
