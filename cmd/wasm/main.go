//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"

	"github.com/notnil/chess"
)

var (
	game           *chess.Game
	boardCanvas    js.Value
	ctx            js.Value
	images         map[string]js.Value
	selectedPos    chess.Square = chess.NoSquare
	ws             js.Value
	currentStyle   = "standard"
	isWhite        = true
	lastMove       *chess.Move
	evalScore      float64      = 0.0
	isGameOver                  = false
	engineDepth                 = 15
	isSetupMode                 = false
	setupSquare    chess.Square = chess.NoSquare
	setupMap       map[chess.Square]chess.Piece
	isMultiplayer  = false
	playerColor    = chess.White
	playerColorMap = map[string]chess.Color{
		"white": chess.White,
		"black": chess.Black,
	}
	multiplayerColor = "white"

	// Animation State
	animMove      *chess.Move
	animPiece     chess.Piece
	animProgress  float64 = 1.0 // 0.0 to 1.0
	animStartTime float64
	animDuration  = 200.0 // milliseconds

	lastHumanSuggestion string

	// Promotion State
	isPromotionMode   bool
	promoSourceSquare chess.Square
	promoTargetSquare chess.Square
)

func main() {
	game = chess.NewGame()
	images = make(map[string]js.Value)
	setupMap = make(map[chess.Square]chess.Piece)

	fmt.Println("WASM: Version 4.8 - High Visibility Status")
	fmt.Println("WASM: Version 5.13 - RC Script Renamed")

	doc := js.Global().Get("document")
	statusLine := doc.Call("getElementById", "wasm-status")
	if !statusLine.IsNull() {
		statusLine.Set("innerText", "WASM v5.13 - READY")
	}

	boardCanvas = doc.Call("getElementById", "chess-board")
	ctx = boardCanvas.Call("getContext", "2d")

	setupWebSocket()
	loadAssets()
	registerEvents()
	updateCapturedPieces()

	requestAnimationFrame := js.Global().Get("requestAnimationFrame")
	var render js.Func
	render = js.FuncOf(func(this js.Value, args []js.Value) any {
		drawBoard()
		requestAnimationFrame.Invoke(render)
		return nil
	})
	requestAnimationFrame.Invoke(render)

	select {}
}
