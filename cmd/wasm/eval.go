//go:build js && wasm

package main

import (
	"fmt"
	"math"
	"syscall/js"
)

func updateEvalBar() {
	doc := js.Global().Get("document")
	fill := doc.Call("getElementById", "eval-white-fill")
	wText := doc.Call("getElementById", "eval-white-text")
	bText := doc.Call("getElementById", "eval-black-text")

	percent := 50.0 + (evalScore * 10.0)
	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}

	fill.Get("style").Set("height", fmt.Sprintf("%f%%", percent))

	// Chess.com analysis style:
	// If White is better (>0), show "+Score" at the bottom.
	// If Black is better (<0), show "-Score" at the top.
	if evalScore > 0 {
		wText.Set("innerText", fmt.Sprintf("+%.1f", evalScore))
		bText.Set("innerText", "")
	} else if evalScore < 0 {
		wText.Set("innerText", "")
		bText.Set("innerText", fmt.Sprintf("-%.1f", math.Abs(evalScore)))
	} else {
		wText.Set("innerText", "0.0")
		bText.Set("innerText", "")
	}
}
