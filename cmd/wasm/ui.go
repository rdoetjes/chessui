//go:build js && wasm

package main

import (
	"fmt"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/notnil/chess"
)

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

type AnimationState struct {
	move      *chess.Move
	piece     chess.Piece
	progress  float64
	startTime float64
	duration  float64
}

func registerEvents() {
	registerBoardEvents()
	registerGameControlEvents()
	registerSettingsEvents()
	registerMultiplayerEvents()
	registerHistoryEvents()
	registerGeminiEvents()
	initPiecePicker()
	registerDebugEvents()
}

func registerBoardEvents() {
	app.ui.boardCanvas.Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		rect := app.ui.boardCanvas.Call("getBoundingClientRect")
		x := event.Get("clientX").Float() - rect.Get("left").Float()
		y := event.Get("clientY").Float() - rect.Get("top").Float()
		size := app.ui.boardCanvas.Get("width").Float()
		tileSize := size / 8

		col := int(x / tileSize)
		row := 7 - int(y/tileSize)

		if app.multi.playerColor == chess.Black {
			col = 7 - col
			row = 7 - row
		}

		handleBoardClick(col, row)
		return nil
	}))
}

func registerGameControlEvents() {
	doc := js.Global().Get("document")

	doc.Call("getElementById", "new-game").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		doc.Call("getElementById", "popup-new-game").Get("classList").Call("remove", "hidden")
		return nil
	}))

	doc.Call("getElementById", "btn-new-game-start").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		colorStr := doc.Call("getElementById", "new-game-color").Get("value").String()
		app.multi.playerColor = playerColorMap[colorStr]
		app.game.isStarted = true

		doc.Call("getElementById", "popup-new-game").Get("classList").Call("add", "hidden")

		app.game.game = chess.NewGame()
		app.game.lastMove = nil
		app.game.isGameOver = false
		app.game.currentViewIndex = -1
		app.game.cachedViewIndex = -2
		app.engine.lastHumanSuggestion = ""
		showSuggestion("")
		doc.Call("getElementById", "engine-info").Set("innerText", "")
		app.game.moveAnnotations = nil
		app.game.initialFEN = ""
		sendUCI("ucinewgame")
		updateMoveList()
		updateCapturedPieces()
		handleBotTurn()
		return nil
	}))

	doc.Call("getElementById", "btn-new-game-cancel").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		doc.Call("getElementById", "popup-new-game").Get("classList").Call("add", "hidden")
		return nil
	}))

	doc.Call("getElementById", "take-back").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		moves := app.game.game.Moves()
		if len(moves) == 0 {
			return nil
		}

		toTakeBack := 2
		if len(moves) < 2 {
			toTakeBack = 1
		}

		newMoves := moves[:len(moves)-toTakeBack]

		if len(app.game.moveAnnotations) >= toTakeBack {
			app.game.moveAnnotations = app.game.moveAnnotations[:len(app.game.moveAnnotations)-toTakeBack]
		} else {
			app.game.moveAnnotations = nil
		}

		if app.game.initialFEN != "" {
			f, _ := chess.FEN(app.game.initialFEN)
			app.game.game = chess.NewGame(f)
		} else {
			app.game.game = chess.NewGame()
		}

		for _, m := range newMoves {
			app.game.game.Move(m)
		}

		app.game.lastMove = nil
		if len(newMoves) > 0 {
			app.game.lastMove = newMoves[len(newMoves)-1]
		}

		app.game.isGameOver = false
		app.game.currentViewIndex = -1
		app.game.cachedViewIndex = -2

		updateMoveList()
		handleBotTurn()
		return nil
	}))

	doc.Call("getElementById", "copy-pgn").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		pgn := fmt.Sprintf("[Event \"Casual Game\"]\n[Site \"chess.phonax.com\"]\n[Date \"??.??.??\"]\n[White \"Player\"]\n[Black \"Stockfish\"]\n[Result \"*\"]\n\n%s", app.game.game.String())
		js.Global().Get("navigator").Get("clipboard").Call("writeText", pgn)

		btn := doc.Call("getElementById", "copy-pgn")
		oldText := btn.Get("innerText").String()
		btn.Set("innerText", "Copied!")
		js.Global().Call("setTimeout", js.FuncOf(func(this js.Value, args []js.Value) any {
			btn.Set("innerText", oldText)
			return nil
		}), 2000)
		return nil
	}))

	doc.Call("getElementById", "import-pgn").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		go func() {
			promise := js.Global().Get("navigator").Get("clipboard").Call("readText")
			success := js.FuncOf(func(this js.Value, args []js.Value) any {
				importText := strings.TrimSpace(args[0].String())
				if importText == "" {
					return nil
				}

				// 1. Try PGN
				f, err := chess.PGN(strings.NewReader(importText))
				if err == nil {
					app.game.game = chess.NewGame(f)
					// Check if we actually got any moves or if it was just a blank PGN
					if len(app.game.game.Moves()) == 0 {
						// If 0 moves, maybe it's a FEN?
						if f2, err2 := chess.FEN(importText); err2 == nil {
							app.game.game = chess.NewGame(f2)
							app.game.initialFEN = importText
						}
					}
				} else {
					// 2. Try FEN if PGN failed
					if f2, err2 := chess.FEN(importText); err2 == nil {
						app.game.game = chess.NewGame(f2)
						app.game.initialFEN = importText
					} else {
						fmt.Printf("WASM: Import Error (PGN: %v, FEN: %v)\n", err, err2)
						js.Global().Call("alert", "Could not parse PGN or FEN")
						return nil
					}
				}

				app.game.moveAnnotations = nil
				app.game.currentViewIndex = -1
				app.game.lastMove = nil
				if moves := app.game.game.Moves(); len(moves) > 0 {
					app.game.lastMove = moves[len(moves)-1]
				}

				app.game.isGameOver = app.game.game.Outcome() != chess.NoOutcome
				app.game.isStarted = true

				updateMoveList()
				updateCapturedPieces()
				handleBotTurn()
				return nil
			})
			promise.Call("then", success)
		}()
		return nil
	}))
}

func registerSettingsEvents() {
	doc := js.Global().Get("document")

	doc.Call("getElementById", "style-select").Set("onchange", js.FuncOf(func(this js.Value, args []js.Value) any {
		app.ui.currentStyle = this.Get("value").String()
		setCookie("chess-style", app.ui.currentStyle, 365)
		applyStyleClass(app.ui.currentStyle)
		loadAssets()
		return nil
	}))

	doc.Call("getElementById", "elo-select").Set("onchange", js.FuncOf(func(this js.Value, args []js.Value) any {
		elo := this.Get("value").String()
		setCookie("chess-elo", elo, 365)
		handleBotTurn()
		return nil
	}))

	doc.Call("getElementById", "setup-board").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		handleSetupToggle()
		return nil
	}))

	doc.Call("getElementById", "close-picker").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		doc.Call("getElementById", "piece-picker").Get("classList").Call("add", "hidden")
		app.ui.isPromotionMode = false
		return nil
	}))

	doc.Call("getElementById", "game-mode").Set("onchange", js.FuncOf(func(this js.Value, args []js.Value) any {
		mode := this.Get("value").String()
		if mode == "bot" {
			reconnectSession("bot-play", "white")
		} else {
			doc.Call("getElementById", "multiplayer-buttons").Get("classList").Call("remove", "hidden")
		}
		return nil
	}))
}

func registerMultiplayerEvents() {
	doc := js.Global().Get("document")

	doc.Call("getElementById", "btn-host-open").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		uuid := generateUUID()
		doc.Call("getElementById", "host-uuid").Set("value", uuid)
		doc.Call("getElementById", "popup-host").Get("classList").Call("remove", "hidden")
		return nil
	}))

	doc.Call("getElementById", "btn-host-start").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		uuid := doc.Call("getElementById", "host-uuid").Get("value").String()
		color := doc.Call("getElementById", "host-color").Get("value").String()
		doc.Call("getElementById", "popup-host").Get("classList").Call("add", "hidden")
		reconnectSession(uuid, color)
		return nil
	}))

	doc.Call("getElementById", "btn-host-cancel").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		doc.Call("getElementById", "popup-host").Get("classList").Call("add", "hidden")
		return nil
	}))

	doc.Call("getElementById", "btn-copy-uuid").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		uuid := doc.Call("getElementById", "host-uuid").Get("value").String()
		js.Global().Get("navigator").Get("clipboard").Call("writeText", uuid)
		return nil
	}))

	doc.Call("getElementById", "btn-join-open").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		doc.Call("getElementById", "popup-join").Get("classList").Call("remove", "hidden")
		return nil
	}))

	doc.Call("getElementById", "btn-join-start").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		uuid := doc.Call("getElementById", "join-uuid").Get("value").String()
		if uuid == "" {
			return nil
		}
		doc.Call("getElementById", "popup-join").Get("classList").Call("add", "hidden")
		reconnectSession(uuid, "auto")
		return nil
	}))

	doc.Call("getElementById", "btn-join-cancel").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		doc.Call("getElementById", "popup-join").Get("classList").Call("add", "hidden")
		return nil
	}))
}

func registerHistoryEvents() {
	doc := js.Global().Get("document")
	doc.Call("getElementById", "move-list").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		target := event.Get("target")
		if !target.Get("dataset").Get("index").IsUndefined() {
			indexStr := target.Get("dataset").Get("index").String()
			index, _ := strconv.Atoi(indexStr)
			jumpToMove(index)
		}
		return nil
	}))
}

func registerDebugEvents() {
	doc := js.Global().Get("document")
	doc.Call("getElementById", "test-sound").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println("WASM: Testing Move Sound...")
		audio := doc.Call("getElementById", "snd-move")
		if !audio.IsNull() {
			audio.Set("currentTime", 0)
			audio.Call("play")
		}
		return nil
	}))
}

func createMoveSpan(doc js.Value, text string, index int, isCurrent bool) js.Value {
	span := doc.Call("createElement", "span")
	span.Get("classList").Call("add", "move-item")
	if isCurrent {
		span.Get("classList").Call("add", "current-move")
	}
	span.Get("dataset").Set("index", index)
	span.Set("innerText", text)
	return span
}

func addGameStartRow(list js.Value, doc js.Value, moves []*chess.Move) {
	div := doc.Call("createElement", "div")
	div.Get("classList").Call("add", "move-row")

	isCurrent := (app.game.currentViewIndex == -1 && len(moves) == 0) || app.game.currentViewIndex == -2
	span := createMoveSpan(doc, "🎬 Game Start", -1, isCurrent)

	div.Call("appendChild", span)
	list.Call("appendChild", div)
}

func getMoveText(i int, moves []*chess.Move) string {
	text := moves[i].String()
	if i < len(app.game.moveAnnotations) && app.game.moveAnnotations[i] != "" {
		text += " " + app.game.moveAnnotations[i]
	}
	return text
}

func addMoveRow(list js.Value, doc js.Value, i int, moves []*chess.Move, highlightIdx int) {
	div := doc.Call("createElement", "div")
	div.Get("classList").Call("add", "move-row")

	// Move Number
	numSpan := doc.Call("createElement", "span")
	numSpan.Set("innerText", fmt.Sprintf("%d. ", (i/2)+1))
	numSpan.Get("style").Set("color", "#888")
	numSpan.Get("style").Set("marginRight", "5px")
	div.Call("appendChild", numSpan)

	// White Move
	whiteText := getMoveText(i, moves)
	div.Call("appendChild", createMoveSpan(doc, whiteText, i, i == highlightIdx))

	// Black Move
	if i+1 < len(moves) {
		div.Call("appendChild", doc.Call("createTextNode", " "))
		blackText := getMoveText(i+1, moves)
		div.Call("appendChild", createMoveSpan(doc, blackText, i+1, i+1 == highlightIdx))
	}

	list.Call("appendChild", div)
}

func updateMoveList() {
	doc := js.Global().Get("document")
	list := doc.Call("getElementById", "move-list")
	list.Set("innerHTML", "")
	moves := app.game.game.Moves()

	addGameStartRow(list, doc, moves)

	highlightIdx := app.game.currentViewIndex
	if highlightIdx == -1 {
		highlightIdx = len(moves) - 1
	}

	for i := 0; i < len(moves); i += 2 {
		addMoveRow(list, doc, i, moves, highlightIdx)
	}

	if app.game.currentViewIndex == -1 {
		list.Set("scrollTop", list.Get("scrollHeight"))
	}
}

func jumpToMove(index int) {
	moves := app.game.game.Moves()
	// If we click the very last move, we go back to "Live" mode
	if index == len(moves)-1 {
		app.game.currentViewIndex = -1
	} else {
		app.game.currentViewIndex = index
	}

	if index == -1 {
		// Special case: if we are at -1 but there ARE moves,
		// we want to show the initial position, so we use a special marker
		// unless the game is actually empty.
		if len(moves) > 0 {
			app.game.currentViewIndex = -2 // Internal marker for "Initial Position"
		}
	}

	// Clear game-over state when navigating history
	app.game.isGameOver = false

	updateMoveList()
	handleBotTurn()
}

func applyStyleClass(style string) {
	doc := js.Global().Get("document")
	body := doc.Get("body")
	body.Set("className", "theme-"+style)
}

func loadAssets() {
	pieces := []string{"wp", "wn", "wb", "wr", "wq", "wk", "bp", "bn", "bb", "br", "bq", "bk", "board"}
	fmt.Printf("WASM: Loading %d assets for style %s...\n", len(pieces), app.ui.currentStyle)
	for _, p := range pieces {
		img := js.Global().Get("Image").New()
		// Use relative path (no leading slash) so it works in subdirectories like /web/
		src := fmt.Sprintf("assets/pieces/%s/%s.png", app.ui.currentStyle, p)
		img.Set("src", src)
		app.ui.images[p] = img

		// Add error logging for images
		img.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
			fmt.Printf("WASM: Failed to load asset: %s\n", src)
			return nil
		}))
		img.Set("onload", js.FuncOf(func(this js.Value, args []js.Value) any {
			// fmt.Printf("WASM: Loaded asset: %s\n", src)
			return nil
		}))
	}
}
