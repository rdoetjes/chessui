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
	engineThreads               = 4
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
	lastPositionEval    float64
	lastMoveSide        chess.Color

	currentViewIndex int = -1 // -1 means viewing the latest move

	// Promotion State
	isPromotionMode   bool
	promoSourceSquare chess.Square
	promoTargetSquare chess.Square

	moveAnnotations []string
	initialFEN      string

	cachedViewGame  *chess.Game
	cachedViewIndex int = -2 // Use -2 to indicate cache is invalid
)

func fetchConfig(callback func()) {
	js.Global().Call("fetch", "/config").
		Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
			return args[0].Call("json")
		})).
		Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
			config := args[0]
			if !config.Get("threads").IsUndefined() {
				engineThreads = config.Get("threads").Int()
				fmt.Printf("WASM: Updated engineThreads to %d from server\n", engineThreads)
			}
			if callback != nil {
				callback()
			}
			return nil
		})).
		Call("catch", js.FuncOf(func(this js.Value, args []js.Value) any {
			fmt.Printf("WASM: Failed to fetch config: %v\n", args[0])
			if callback != nil {
				callback()
			}
			return nil
		}))
}

func main() {
	game = chess.NewGame()
	images = make(map[string]js.Value)
	setupMap = make(map[chess.Square]chess.Piece)

	fmt.Println("Stockfish Chess UI Light - v1.0.2")

	doc := js.Global().Get("document")
	statusLine := doc.Call("getElementById", "wasm-status")
	if !statusLine.IsNull() {
		statusLine.Set("innerText", " READY")
	}

	boardCanvas = doc.Call("getElementById", "chess-board")
	ctx = boardCanvas.Call("getContext", "2d")

	setupWebSocket()

	// Load style from cookie
	savedStyle := getCookie("chess-style")
	if savedStyle != "" {
		// Update dropdown value
		doc.Call("getElementById", "style-select").Set("value", savedStyle)
	}

	// Load ELO from cookie
	savedElo := getCookie("chess-elo")
	if savedElo != "" {
		// Update dropdown value
		doc.Call("getElementById", "elo-select").Set("value", savedElo)
	}

	loadAssets()
	applyStyleClass(savedStyle)
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
