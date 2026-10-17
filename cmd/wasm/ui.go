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
	doc := js.Global().Get("document")

	// Canvas Click
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

	// Controls
	doc.Call("getElementById", "new-game").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		app.game.game = chess.NewGame()
		app.game.lastMove = nil
		app.game.isGameOver = false
		app.game.currentViewIndex = -1
		app.game.cachedViewIndex = -2
		app.engine.lastHumanSuggestion = ""                            // Reset suggestion data
		showSuggestion("")                                             // Clear suggestion UI
		doc.Call("getElementById", "engine-info").Set("innerText", "") // Clear engine info
		doc.Call("getElementById", "checkmate-alert").Get("classList").Call("add", "hidden")
		app.game.moveAnnotations = nil
		app.game.initialFEN = ""
		sendUCI("ucinewgame")
		updateMoveList()
		updateCapturedPieces()
		handleBotTurn()
		return nil
	}))

	doc.Call("getElementById", "take-back").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		moves := app.game.game.Moves()
		if len(moves) == 0 {
			return nil
		}

		// Take back two moves: the bot's move and the player's move
		// If only one move has been played, take back just one.
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

		// Reset game and re-apply moves
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
		doc.Call("getElementById", "checkmate-alert").Get("classList").Call("add", "hidden")

		updateMoveList()
		handleBotTurn() // Update engine with new position
		return nil
	}))

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
			// Show the Host/Join buttons
			doc.Call("getElementById", "multiplayer-buttons").Get("classList").Call("remove", "hidden")
		}
		return nil
	}))

	// Host Game
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

	// Join Game
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
		// Join as "auto" - server or client will determine opposite color
		reconnectSession(uuid, "auto")
		return nil
	}))

	doc.Call("getElementById", "btn-join-cancel").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		doc.Call("getElementById", "popup-join").Get("classList").Call("add", "hidden")
		return nil
	}))

	doc.Call("getElementById", "copy-pgn").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		// Create a PGN with standard headers
		pgn := fmt.Sprintf("[Event \"Casual Game\"]\n[Site \"chess.phonax.com\"]\n[Date \"??.??.??\"]\n[White \"Player\"]\n[Black \"Stockfish\"]\n[Result \"*\"]\n\n%s", app.game.game.String())

		js.Global().Get("navigator").Get("clipboard").Call("writeText", pgn)

		// Optional: Change button text briefly to show success
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
				pgnText := args[0].String()
				if pgnText == "" {
					return nil
				}

				f, err := chess.PGN(strings.NewReader(pgnText))
				if err != nil {
					fmt.Printf("WASM: PGN Import Error: %v\n", err)
					return nil
				}

				app.game.game = chess.NewGame(f)
				app.game.moveAnnotations = nil
				app.game.currentViewIndex = -1 // Go to latest move
				app.game.lastMove = nil
				if moves := app.game.game.Moves(); len(moves) > 0 {
					app.game.lastMove = moves[len(moves)-1]
				}

				app.game.isGameOver = false
				updateMoveList()
				updateCapturedPieces()
				handleBotTurn()
				return nil
			})
			promise.Call("then", success)
		}()
		return nil
	}))

	// Move List Click (Event Delegation)
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

	initPiecePicker()

	// Test Sound Debug Button
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

func updateMoveList() {
	doc := js.Global().Get("document")
	list := doc.Call("getElementById", "move-list")
	list.Set("innerHTML", "")
	moves := app.game.game.Moves()

	// Initial Position item
	startDiv := doc.Call("createElement", "div")
	startDiv.Get("classList").Call("add", "move-row")
	startSpan := doc.Call("createElement", "span")
	startSpan.Get("classList").Call("add", "move-item")
	if app.game.currentViewIndex == -1 && len(moves) == 0 {
		startSpan.Get("classList").Call("add", "current-move")
	} else if app.game.currentViewIndex == -2 { // -2 means we clicked the start
		startSpan.Get("classList").Call("add", "current-move")
	}
	startSpan.Get("dataset").Set("index", "-1")
	startSpan.Set("innerText", "🎬 Game Start")
	startDiv.Call("appendChild", startSpan)
	list.Call("appendChild", startDiv)

	// If currentViewIndex is -1, we are viewing the latest move
	highlightIdx := app.game.currentViewIndex
	if highlightIdx == -1 {
		highlightIdx = len(moves) - 1
	}

	for i := 0; i < len(moves); i += 2 {
		div := doc.Call("createElement", "div")
		div.Get("classList").Call("add", "move-row")

		// Move Number
		numSpan := doc.Call("createElement", "span")
		numSpan.Set("innerText", fmt.Sprintf("%d. ", (i/2)+1))
		numSpan.Get("style").Set("color", "#888")
		numSpan.Get("style").Set("marginRight", "5px")
		div.Call("appendChild", numSpan)

		// Move 1
		m1 := moves[i].String()
		if i < len(app.game.moveAnnotations) && app.game.moveAnnotations[i] != "" {
			m1 += " " + app.game.moveAnnotations[i]
		}
		s1 := doc.Call("createElement", "span")
		s1.Get("classList").Call("add", "move-item")
		if i == highlightIdx {
			s1.Get("classList").Call("add", "current-move")
		}
		s1.Get("dataset").Set("index", i)
		s1.Set("innerText", m1)
		div.Call("appendChild", s1)

		// Move 2
		if i+1 < len(moves) {
			// Spacer
			div.Call("appendChild", doc.Call("createTextNode", " "))

			m2 := moves[i+1].String()
			if i+1 < len(app.game.moveAnnotations) && app.game.moveAnnotations[i+1] != "" {
				m2 += " " + app.game.moveAnnotations[i+1]
			}
			s2 := doc.Call("createElement", "span")
			s2.Get("classList").Call("add", "move-item")
			if i+1 == highlightIdx {
				s2.Get("classList").Call("add", "current-move")
			}
			s2.Get("dataset").Set("index", i+1)
			s2.Set("innerText", m2)
			div.Call("appendChild", s2)
		}

		list.Call("appendChild", div)
	}
	// Scroll to bottom only if we're at the latest move
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
	doc := js.Global().Get("document")
	doc.Call("getElementById", "checkmate-alert").Get("classList").Call("add", "hidden")

	updateMoveList()

	// Only trigger bot if we are back in "Live" mode
	if app.game.currentViewIndex == -1 {
		handleBotTurn()
	}
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
