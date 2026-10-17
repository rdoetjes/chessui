//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"

	"github.com/notnil/chess"
)

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
	isWhite          bool // Board orientation
}

type UIState struct {
	boardCanvas       js.Value
	ctx               js.Value
	images            map[string]js.Value
	currentStyle      string
	evalScore         float64
	isPromotionMode   bool
	promoSourceSquare chess.Square
	promoTargetSquare chess.Square
}

type EngineState struct {
	threads             int
	lastHumanSuggestion string
	lastPositionEval    float64
	lastMoveSide        chess.Color
}

type AnimationState struct {
	move      *chess.Move
	piece     chess.Piece
	progress  float64
	startTime float64
	duration  float64
}

type MultiplayerState struct {
	ws               js.Value
	isActive         bool
	playerColor      chess.Color
	multiplayerColor string
}

type App struct {
	game   GameState
	ui     UIState
	engine EngineState
	anim   AnimationState
	multi  MultiplayerState
}

var app = App{
	game: GameState{
		selectedPos:      chess.NoSquare,
		currentViewIndex: -1,
		cachedViewIndex:  -2,
		setupMap:         make(map[chess.Square]chess.Piece),
		isWhite:          true,
	},
	ui: UIState{
		images:       make(map[string]js.Value),
		currentStyle: "standard",
	},
	engine: EngineState{
		threads: 4,
	},
	anim: AnimationState{
		progress: 1.0,
		duration: 200.0,
	},
	multi: MultiplayerState{
		playerColor:      chess.White,
		multiplayerColor: "white",
	},
}

var playerColorMap = map[string]chess.Color{
	"white": chess.White,
	"black": chess.Black,
}

func fetchConfig(callback func()) {
	js.Global().Call("fetch", "/config").
		Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
			return args[0].Call("json")
		})).
		Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
			config := args[0]
			if !config.Get("threads").IsUndefined() {
				app.engine.threads = config.Get("threads").Int()
				fmt.Printf("WASM: Updated engineThreads to %d from server\n", app.engine.threads)
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
	app.game.game = chess.NewGame()

	fmt.Println("Stockfish Chess UI Light - v1.0.2")

	doc := js.Global().Get("document")
	statusLine := doc.Call("getElementById", "wasm-status")
	if !statusLine.IsNull() {
		statusLine.Set("innerText", " READY")
	}

	app.ui.boardCanvas = doc.Call("getElementById", "chess-board")
	app.ui.ctx = app.ui.boardCanvas.Call("getContext", "2d")

	setupWebSocket()

	// Load style from cookie
	savedStyle := getCookie("chess-style")
	if savedStyle != "" {
		app.ui.currentStyle = savedStyle
		// Update dropdown value
		doc.Call("getElementById", "style-select").Set("value", app.ui.currentStyle)
	}

	// Load ELO from cookie
	savedElo := getCookie("chess-elo")
	if savedElo != "" {
		// Update dropdown value
		doc.Call("getElementById", "elo-select").Set("value", savedElo)
	}

	loadAssets()
	applyStyleClass(app.ui.currentStyle)
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
