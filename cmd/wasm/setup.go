//go:build js && wasm

package main

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/notnil/chess"
)

func handleSetupToggle() {
	doc := js.Global().Get("document")
	btn := doc.Call("getElementById", "setup-board")

	if !isSetupMode {
		isSetupMode = true
		btn.Set("innerText", "Play from Position")
		btn.Get("style").Set("backgroundColor", "#f44336")
		doc.Call("getElementById", "setup-controls").Get("classList").Call("remove", "hidden")

		setupMap = make(map[chess.Square]chess.Piece)
		f, _ := chess.FEN("8/8/8/8/8/8/8/8 w - - 0 1")
		game = chess.NewGame(f)
		lastMove = nil
		moveAnnotations = nil
		updateMoveList()
	} else {
		wKing, bKing := false, false
		for _, p := range setupMap {
			if p == chess.WhiteKing {
				wKing = true
			}
			if p == chess.BlackKing {
				bKing = true
			}
		}
		if !wKing || !bKing {
			js.Global().Call("alert", "Error: Place both kings!")
			return
		}

		isSetupMode = false
		btn.Set("innerText", "Setup Board")
		btn.Get("style").Set("backgroundColor", "")
		doc.Call("getElementById", "setup-controls").Get("classList").Call("add", "hidden")

		turn := doc.Call("getElementById", "turn-select").Get("value").String()
		f, _ := chess.FEN(generateFEN(setupMap, turn))
		game = chess.NewGame(f)
		moveAnnotations = nil
		handleBotTurn()
		updateMoveList()
	}
}

func initPiecePicker() {
	doc := js.Global().Get("document")
	picker := doc.Call("getElementById", "piece-picker").Call("querySelector", ".picker-grid")
	picker.Set("innerHTML", "")
	pieces := []string{"wp", "wn", "wb", "wr", "wq", "wk", "bp", "bn", "bb", "br", "bq", "bk", "none"}
	for _, p := range pieces {
		div := doc.Call("createElement", "div")
		div.Get("classList").Call("add", "picker-piece")
		if strings.HasPrefix(p, "b") {
			div.Get("classList").Call("add", "black-piece-bg")
		}
		if strings.HasPrefix(p, "w") {
			div.Get("classList").Call("add", "white-piece-bg")
		}

		if p != "none" {
			img := doc.Call("createElement", "img")
			img.Set("src", fmt.Sprintf("/assets/pieces/%s/%s.png", currentStyle, p))
			div.Call("appendChild", img)
		} else {
			div.Set("innerText", "X")
		}

		pt := p
		div.Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
			handlePickerSelect(pt)
			return nil
		}))
		picker.Call("appendChild", div)
	}
}

func handlePickerSelect(pStr string) {
	if isPromotionMode {
		completePromotion(pStr)
		return
	}

	var piece chess.Piece = chess.NoPiece
	switch pStr {
	case "wp":
		piece = chess.WhitePawn
	case "wn":
		piece = chess.WhiteKnight
	case "wb":
		piece = chess.WhiteBishop
	case "wr":
		piece = chess.WhiteRook
	case "wq":
		piece = chess.WhiteQueen
	case "wk":
		piece = chess.WhiteKing
	case "bp":
		piece = chess.BlackPawn
	case "bn":
		piece = chess.BlackKnight
	case "bb":
		piece = chess.BlackBishop
	case "br":
		piece = chess.BlackRook
	case "bq":
		piece = chess.BlackQueen
	case "bk":
		piece = chess.BlackKing
	}

	if piece != chess.NoPiece && piece.Type() == chess.King {
		for sq, p := range setupMap {
			if p.Type() == chess.King && p.Color() == piece.Color() && sq != setupSquare {
				js.Global().Call("alert", "Only one king allowed!")
				return
			}
		}
	}

	if piece == chess.NoPiece {
		delete(setupMap, setupSquare)
	} else {
		setupMap[setupSquare] = piece
	}

	// Fast board update during setup
	f, _ := chess.FEN(generateFEN(setupMap, "w"))
	game = chess.NewGame(f)

	js.Global().Get("document").Call("getElementById", "piece-picker").Get("classList").Call("add", "hidden")
}

func showPromotionPicker(color chess.Color) {
	doc := js.Global().Get("document")
	picker := doc.Call("getElementById", "piece-picker").Call("querySelector", ".picker-grid")
	picker.Set("innerHTML", "")

	cStr := "w"
	if color == chess.Black {
		cStr = "b"
	}

	pieces := []string{cStr + "q", cStr + "r", cStr + "b", cStr + "n"}
	for _, p := range pieces {
		div := doc.Call("createElement", "div")
		div.Get("classList").Call("add", "picker-piece")
		if color == chess.Black {
			div.Get("classList").Call("add", "black-piece-bg")
		} else {
			div.Get("classList").Call("add", "white-piece-bg")
		}

		img := doc.Call("createElement", "img")
		img.Set("src", fmt.Sprintf("/assets/pieces/%s/%s.png", currentStyle, p))
		div.Call("appendChild", img)

		pt := p
		div.Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
			handlePickerSelect(pt)
			return nil
		}))
		picker.Call("appendChild", div)
	}

	doc.Call("getElementById", "piece-picker").Get("classList").Call("remove", "hidden")
}

func completePromotion(pStr string) {
	promoChar := string(pStr[1]) // q, r, b, n
	moveStr := fmt.Sprintf("%s%s%s", promoSourceSquare.String(), promoTargetSquare.String(), promoChar)

	isPromotionMode = false
	js.Global().Get("document").Call("getElementById", "piece-picker").Get("classList").Call("add", "hidden")

	executeMove(moveStr, true)
}
