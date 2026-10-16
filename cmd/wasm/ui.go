//go:build js && wasm

package main

import (
	"fmt"
	"strconv"
	"syscall/js"

	"github.com/notnil/chess"
)

func registerEvents() {
	doc := js.Global().Get("document")

	// Canvas Click
	boardCanvas.Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		rect := boardCanvas.Call("getBoundingClientRect")
		x := event.Get("clientX").Float() - rect.Get("left").Float()
		y := event.Get("clientY").Float() - rect.Get("top").Float()
		size := boardCanvas.Get("width").Float()
		tileSize := size / 8

		col := int(x / tileSize)
		row := 7 - int(y/tileSize)

		if playerColor == chess.Black {
			col = 7 - col
			row = 7 - row
		}

		handleBoardClick(col, row)
		return nil
	}))

	// Controls
	doc.Call("getElementById", "new-game").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		game = chess.NewGame()
		lastMove = nil
		isGameOver = false
		lastHumanSuggestion = ""                                       // Reset suggestion data
		showSuggestion("")                                             // Clear suggestion UI
		doc.Call("getElementById", "engine-info").Set("innerText", "") // Clear engine info
		doc.Call("getElementById", "checkmate-alert").Get("classList").Call("add", "hidden")
		moveAnnotations = nil
		initialFEN = ""
		sendUCI("ucinewgame")
		updateMoveList()
		updateCapturedPieces()
		handleBotTurn()
		return nil
	}))

	doc.Call("getElementById", "take-back").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		moves := game.Moves()
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

		if len(moveAnnotations) >= toTakeBack {
			moveAnnotations = moveAnnotations[:len(moveAnnotations)-toTakeBack]
		} else {
			moveAnnotations = nil
		}

		// Reset game and re-apply moves
		if initialFEN != "" {
			f, _ := chess.FEN(initialFEN)
			game = chess.NewGame(f)
		} else {
			game = chess.NewGame()
		}

		for _, m := range newMoves {
			game.Move(m)
		}

		lastMove = nil
		if len(newMoves) > 0 {
			lastMove = newMoves[len(newMoves)-1]
		}

		isGameOver = false
		doc.Call("getElementById", "checkmate-alert").Get("classList").Call("add", "hidden")

		updateMoveList()
		handleBotTurn() // Update engine with new position
		return nil
	}))

	doc.Call("getElementById", "style-select").Set("onchange", js.FuncOf(func(this js.Value, args []js.Value) any {
		currentStyle = this.Get("value").String()
		applyStyleClass(currentStyle)
		loadAssets()
		return nil
	}))

	doc.Call("getElementById", "elo-select").Set("onchange", js.FuncOf(func(this js.Value, args []js.Value) any {
		handleBotTurn()
		return nil
	}))

	doc.Call("getElementById", "setup-board").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		handleSetupToggle()
		return nil
	}))

	doc.Call("getElementById", "close-picker").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		doc.Call("getElementById", "piece-picker").Get("classList").Call("add", "hidden")
		isPromotionMode = false
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
		pgn := fmt.Sprintf("[Event \"Casual Game\"]\n[Site \"chess.phonax.com\"]\n[Date \"??.??.??\"]\n[White \"Player\"]\n[Black \"Stockfish\"]\n[Result \"*\"]\n\n%s", game.String())

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
}

func updateMoveList() {
	doc := js.Global().Get("document")
	list := doc.Call("getElementById", "move-list")
	list.Set("innerHTML", "")
	moves := game.Moves()
	currentMoveIdx := len(moves) - 1

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
		if i < len(moveAnnotations) && moveAnnotations[i] != "" {
			m1 += " " + moveAnnotations[i]
		}
		s1 := doc.Call("createElement", "span")
		s1.Get("classList").Call("add", "move-item")
		if i == currentMoveIdx {
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
			if i+1 < len(moveAnnotations) && moveAnnotations[i+1] != "" {
				m2 += " " + moveAnnotations[i+1]
			}
			s2 := doc.Call("createElement", "span")
			s2.Get("classList").Call("add", "move-item")
			if i+1 == currentMoveIdx {
				s2.Get("classList").Call("add", "current-move")
			}
			s2.Get("dataset").Set("index", i+1)
			s2.Set("innerText", m2)
			div.Call("appendChild", s2)
		}

		list.Call("appendChild", div)
	}
	// Scroll to bottom
	list.Set("scrollTop", list.Get("scrollHeight"))
}

func jumpToMove(index int) {
	allMoves := game.Moves()
	if index < 0 || index >= len(allMoves) {
		return
	}

	// We only keep moves up to the point we clicked
	newMoves := allMoves[:index+1]

	// Truncate annotations to match
	if len(moveAnnotations) > len(newMoves) {
		moveAnnotations = moveAnnotations[:len(newMoves)]
	}

	// Reset game and re-apply moves
	if initialFEN != "" {
		f, _ := chess.FEN(initialFEN)
		game = chess.NewGame(f)
	} else {
		game = chess.NewGame()
	}

	for _, m := range newMoves {
		game.Move(m)
	}

	// Update UI state
	lastMove = nil
	if len(newMoves) > 0 {
		lastMove = newMoves[len(newMoves)-1]
	}

	isGameOver = false
	doc := js.Global().Get("document")
	doc.Call("getElementById", "checkmate-alert").Get("classList").Call("add", "hidden")

	updateMoveList()
	updateCapturedPieces()
	handleBotTurn()
}

func applyStyleClass(style string) {
	doc := js.Global().Get("document")
	body := doc.Get("body")
	body.Set("className", "theme-"+style)
}

func loadAssets() {
	pieces := []string{"wp", "wn", "wb", "wr", "wq", "wk", "bp", "bn", "bb", "br", "bq", "bk", "board"}
	fmt.Printf("WASM: Loading %d assets for style %s...\n", len(pieces), currentStyle)
	for _, p := range pieces {
		img := js.Global().Get("Image").New()
		// Use relative path (no leading slash) so it works in subdirectories like /web/
		src := fmt.Sprintf("assets/pieces/%s/%s.png", currentStyle, p)
		img.Set("src", src)
		images[p] = img

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
