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
		handleBoardClick(col, row)
		return nil
	}))

	// Controls
	doc.Call("getElementById", "new-game").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		game = chess.NewGame()
		lastMove = nil
		isGameOver = false
		doc.Call("getElementById", "checkmate-alert").Get("classList").Call("add", "hidden")
		sendUCI("ucinewgame")
		updateMoveList()
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
		sendMoveToStockfish() // Update engine with new position
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
		text := fmt.Sprintf("%d. %s", (i/2)+1, moves[i].String())
		if i+1 < len(moves) {
			text += " " + moves[i+1].String()
		}
		div.Set("innerText", text)
		list.Call("appendChild", div)
	}
}

func loadAssets() {
	pieces := []string{"wp", "wn", "wb", "wr", "wq", "wk", "bp", "bn", "bb", "br", "bq", "bk", "board"}
	for _, p := range pieces {
		img := js.Global().Get("Image").New()
		img.Set("src", fmt.Sprintf("/assets/pieces/%s/%s.png", currentStyle, p))
		images[p] = img
	}
}
