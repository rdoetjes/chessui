//go:build js && wasm

package main

import (
	"fmt"
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
		game = chess.NewGame()
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
		loadAssets()
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

	initPiecePicker()
}

func updateMoveList() {
	doc := js.Global().Get("document")
	list := doc.Call("getElementById", "move-list")
	list.Set("innerHTML", "")
	moves := game.Moves()
	for i := 0; i < len(moves); i += 2 {
		div := doc.Call("createElement", "div")
		div.Get("classList").Call("add", "move-row")

		// Move 1
		m1 := moves[i].String()
		if i < len(moveAnnotations) && moveAnnotations[i] != "" {
			m1 += " " + moveAnnotations[i]
		}

		text := fmt.Sprintf("%d. %s", (i/2)+1, m1)

		// Move 2
		if i+1 < len(moves) {
			m2 := moves[i+1].String()
			if i+1 < len(moveAnnotations) && moveAnnotations[i+1] != "" {
				m2 += " " + moveAnnotations[i+1]
			}
			text += " " + m2
		}
		div.Set("innerText", text)
		list.Call("appendChild", div)
	}
	// Scroll to bottom
	list.Set("scrollTop", list.Get("scrollHeight"))
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
