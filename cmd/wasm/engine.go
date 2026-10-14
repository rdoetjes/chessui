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
	wsURL := "ws://" + host + "/ws"
	if loc.Get("protocol").String() == "https:" {
		wsURL = "wss://" + host + "/ws"
	}

	doc := js.Global().Get("document")
	infoPanel := doc.Call("getElementById", "engine-info")
	infoPanel.Set("innerText", "Connecting...")

	ws = js.Global().Get("WebSocket").New(wsURL)

	ws.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println("WASM: Engine Linked")
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

func handleUCIResponse(msg string) {
	info := uci.ParseLine(msg)

	// Normalize evaluation to White's perspective
	if info.Score.CP != 0 || info.Score.IsMate || strings.Contains(msg, "score cp 0") {
		score := float64(info.Score.CP) / 100.0
		if info.Score.IsMate {
			score = 10.0
			if info.Score.Mate < 0 {
				score = -10.0
			}
		}

		// If it's Black's turn, the engine score 'cp 100' means Black is up 1.0.
		// To keep White perspective (+ is white better), we negate if it's Black's turn.
		if game.Position().Turn() == chess.Black {
			score = -score
		}

		evalScore = score
		updateEvalBar()
	}

	if info.BestMove != "" && !isSetupMode && !isGameOver {
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
