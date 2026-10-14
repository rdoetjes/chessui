//go:build js && wasm

package main

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/notnil/chess"
	"github.com/rdoetjes/chess-ui/internal/uci"
)

func setupWebSocket() {
	loc := js.Global().Get("location")
	host := loc.Get("host").String()

	params := js.Global().Get("URLSearchParams").New(loc.Get("search"))
	roomID := params.Call("get", "room").String()
	if roomID == "null" {
		roomID = "lobby"
	}

	wsURL := "ws://" + host + "/ws?room=" + roomID
	if loc.Get("protocol").String() == "https:" {
		wsURL = "wss://" + host + "/ws?room=" + roomID
	}

	fmt.Printf("WASM: Joining Room %s\n", roomID)

	doc := js.Global().Get("document")
	infoPanel := doc.Call("getElementById", "engine-info")
	infoPanel.Set("innerText", "Joining Room: "+roomID)

	ws = js.Global().Get("WebSocket").New(wsURL)

	ws.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println("WASM: Connected to Room")
		infoPanel.Set("innerText", "")
		sendUCI("uci")
		sendUCI("isready")
		return nil
	}))

	ws.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		handleUCIResponse(args[0].Get("data").String())
		return nil
	}))
}

func reconnectSession(roomID string) {
	if ws.Get("readyState").Int() == 1 {
		ws.Call("close")
	}

	loc := js.Global().Get("location")
	host := loc.Get("host").String()
	wsURL := "ws://" + host + "/ws?room=" + roomID
	if loc.Get("protocol").String() == "https:" {
		wsURL = "wss://" + host + "/ws?room=" + roomID
	}

	fmt.Printf("WASM: Switching to Room %s\n", roomID)

	doc := js.Global().Get("document")
	infoPanel := doc.Call("getElementById", "engine-info")
	infoPanel.Set("innerText", "Joining: "+roomID)

	ws = js.Global().Get("WebSocket").New(wsURL)

	ws.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println("WASM: Connected to Session")
		infoPanel.Set("innerText", "")
		sendUCI("uci")
		sendUCI("isready")
		// Sync current board if we joined an existing game
		sendMoveToStockfish()
		return nil
	}))

	ws.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		handleUCIResponse(args[0].Get("data").String())
		return nil
	}))
}

func handleUCIResponse(msg string) {
	if strings.HasPrefix(msg, "opponent_move ") {
		moveStr := strings.TrimPrefix(msg, "opponent_move ")
		for _, m := range game.ValidMoves() {
			if m.String() == moveStr {
				game.Move(m)
				lastMove = m
				updateMoveList()
				checkGameState()
				// Send to local Stockfish to keep eval in sync
				sendUCI("position fen " + game.Position().String())
				break
			}
		}
		return
	}

	info := uci.ParseLine(msg)

	if info.Score.CP != 0 || info.Score.IsMate || strings.Contains(msg, "score cp 0") {
		score := float64(info.Score.CP) / 100.0
		if info.Score.IsMate {
			score = 10.0
			if info.Score.Mate < 0 {
				score = -10.0
			}
		}
		if game.Position().Turn() == chess.Black {
			score = -score
		}
		evalScore = score
		updateEvalBar()
	}

	// Only play bot moves if NOT in multiplayer mode
	if info.BestMove != "" && !isSetupMode && !isGameOver && !isMultiplayer {
		for _, m := range game.ValidMoves() {
			if m.String() == info.BestMove {
				game.Move(m)
				lastMove = m
				updateMoveList()
				checkGameState()
				break
			}
		}
	}
}

func sendMoveToStockfish() {
	moveStr := ""
	if lastMove != nil {
		moveStr = lastMove.String()
	}
	if moveStr != "" {
		sendUCI("move " + moveStr)
	}

	sendUCI("position fen " + game.Position().String())
	elo := js.Global().Get("document").Call("getElementById", "elo-select").Get("value").String()
	sendUCI("setoption name UCI_LimitStrength value true")
	sendUCI("setoption name UCI_Elo value " + elo)
	sendUCI(fmt.Sprintf("go depth %d", engineDepth))
}

func sendUCI(msg string) {
	if ws.Get("readyState").Int() == 1 {
		ws.Call("send", msg)
	}
}
