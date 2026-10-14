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

	if fill.IsNull() {
		return
	}

	// Chess.com style mapping: use a sigmoid-like curve for better visualization
	// instead of just score * 10
	score := evalScore

	// Convert score to percentage (0 to 100)
	// At 0.0, percent is 50
	// At +4.0, percent is ~90
	// At -4.0, percent is ~10
	percent := 50.0 + (math.Atan(score/3.0) * (100.0 / math.Pi))

	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}

	// Update the White fill height (fills from bottom)
	fill.Get("style").Set("height", fmt.Sprintf("%.1f%%", percent))

	// Chess.com analysis style labels:
	// If White is better (>0), show score at bottom (on white fill).
	// If Black is better (<0), show score at top (on black background).
	if score >= 0 {
		wText.Set("innerText", fmt.Sprintf("+%.1f", score))
		bText.Set("innerText", "")
		// If very winning for white, make text black on white fill
		if percent > 80 {
			wText.Get("style").Set("color", "#000")
		} else {
			wText.Get("style").Set("color", "#fff")
		}
	} else {
		wText.Set("innerText", "")
		bText.Set("innerText", fmt.Sprintf("-%.1f", math.Abs(score)))
		// If very winning for black, make text white on black background
		if percent < 20 {
			bText.Get("style").Set("color", "#fff")
		} else {
			bText.Get("style").Set("color", "#fff")
		}
	}
}
