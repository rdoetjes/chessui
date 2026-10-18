//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
)

func registerGeminiEvents() {
	doc := js.Global().Get("document")

	doc.Call("getElementById", "analyze-game").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		key := getCookie("gemini-api-key")
		if key == "" {
			doc.Call("getElementById", "popup-gemini").Get("classList").Call("remove", "hidden")
		} else {
			startAnalysis(key)
		}
		return nil
	}))

	doc.Call("getElementById", "btn-gemini-ok").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		key := doc.Call("getElementById", "gemini-key").Get("value").String()
		if key != "" {
			setCookie("gemini-api-key", key, 365)
			doc.Call("getElementById", "popup-gemini").Get("classList").Call("add", "hidden")
			startAnalysis(key)
		}
		return nil
	}))

	doc.Call("getElementById", "btn-gemini-cancel").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		doc.Call("getElementById", "popup-gemini").Get("classList").Call("add", "hidden")
		return nil
	}))

	doc.Call("getElementById", "btn-analysis-close").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		doc.Call("getElementById", "popup-analysis").Get("classList").Call("add", "hidden")
		return nil
	}))
}

func startAnalysis(apiKey string) {
	doc := js.Global().Get("document")

	// Remove game over overlay if present
	app.game.isGameOver = false

	doc.Call("getElementById", "popup-analysis").Get("classList").Call("remove", "hidden")
	doc.Call("getElementById", "analysis-status").Get("classList").Call("remove", "hidden")
	doc.Call("getElementById", "analysis-status").Set("innerText", "Analyzing game with Gemini AI...")
	doc.Call("getElementById", "analysis-results").Get("classList").Call("add", "hidden")
	doc.Call("getElementById", "analysis-results").Set("innerHTML", "")

	pgn := fmt.Sprintf("[Event \"Casual Game\"]\n[Site \"chess.phonax.com\"]\n[Date \"??.??.??\"]\n[White \"Player\"]\n[Black \"Bot\"]\n[Result \"*\"]\n\n%s", app.game.game.String())

	go func() {
		analysis, err := fetchGeminiAnalysis(apiKey, pgn)
		if err != nil {
			doc.Call("getElementById", "analysis-status").Set("innerText", "Error: "+err.Error())
			return
		}

		displayAnalysis(analysis)
	}()
}

type GeminiAnalysis struct {
	GoodMoves []MoveAnalysis `json:"good_moves"`
	BadMoves  []MoveAnalysis `json:"bad_moves"`
	Tips      []string       `json:"tips"`
}

type MoveAnalysis struct {
	Move        string `json:"move"`
	Explanation string `json:"explanation"`
}

func fetchGeminiAnalysis(apiKey, pgn string) (*GeminiAnalysis, error) {
	prompt := fmt.Sprintf(`You are a professional chess coach. Analyze the following chess game in PGN format. Identify 2-3 key good moves and 2-3 key bad moves (mistakes or blunders). For each, explain simply why it was good or bad. Finally, give 3 concise tips for the player to improve in their next game.

PGN:
%s

Format your response ONLY as a JSON object with the following structure:
{
  "good_moves": [{"move": "move in algebraic notation", "explanation": "short explanation"}],
  "bad_moves": [{"move": "move in algebraic notation", "explanation": "short explanation"}],
  "tips": ["tip 1", "tip 2", "tip 3"]
}`, pgn)

	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=" + apiKey

	payload := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]any{
					{
						"text": prompt,
					},
				},
			},
		},
		"generationConfig": map[string]any{
			"response_mime_type": "application/json",
		},
	}

	jsonPayload, _ := json.Marshal(payload)

	// Use JS fetch
	promise := js.Global().Call("fetch", url, map[string]any{
		"method": "POST",
		"body":   string(jsonPayload),
		"headers": map[string]any{
			"Content-Type": "application/json",
		},
	})

	// Wait for promise
	ch := make(chan js.Value)
	promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- args[0]
		return nil
	})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- args[0]
		return nil
	}))

	resp := <-ch
	if resp.Type() == js.TypeObject {
		ok := resp.Get("ok")
		if !ok.IsUndefined() && ok.Bool() {
			// Read body
			textPromise := resp.Call("text")
			textCh := make(chan string)
			textPromise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
				textCh <- args[0].String()
				return nil
			}))

			bodyText := <-textCh

			var geminiResp struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"content"`
				} `json:"candidates"`
				Error struct {
					Message string `json:"message"`
					Status  string `json:"status"`
				} `json:"error"`
			}

			err := json.Unmarshal([]byte(bodyText), &geminiResp)
			if err != nil {
				return nil, fmt.Errorf("failed to parse Gemini response: %v", err)
			}

			if geminiResp.Error.Message != "" {
				return nil, fmt.Errorf("Gemini API Error: %s (%s)", geminiResp.Error.Message, geminiResp.Error.Status)
			}

			if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
				return nil, fmt.Errorf("empty response from Gemini")
			}

			var analysis GeminiAnalysis
			err = json.Unmarshal([]byte(geminiResp.Candidates[0].Content.Parts[0].Text), &analysis)
			if err != nil {
				// Fallback: Gemini might have returned plain text instead of JSON if it ignored the directive
				return nil, fmt.Errorf("failed to parse analysis JSON. Body: %s", geminiResp.Candidates[0].Content.Parts[0].Text)
			}

			return &analysis, nil
		}

		statusText := resp.Get("statusText").String()
		status := resp.Get("status").Int()
		if statusText == "" {
			statusText = "Unknown Error"
		}
		return nil, fmt.Errorf("API request failed with status: %d (%s)", status, statusText)
	}

	return nil, fmt.Errorf("failed to reach Gemini API")
}

func displayAnalysis(analysis *GeminiAnalysis) {
	doc := js.Global().Get("document")
	resultsDiv := doc.Call("getElementById", "analysis-results")
	statusDiv := doc.Call("getElementById", "analysis-status")

	statusDiv.Get("classList").Call("add", "hidden")
	resultsDiv.Get("classList").Call("remove", "hidden")
	resultsDiv.Set("innerHTML", "")

	// Good Moves
	if len(analysis.GoodMoves) > 0 {
		h4 := doc.Call("createElement", "h4")
		h4.Set("innerText", "🟢 Good Moves")
		h4.Get("style").Set("color", "#4caf50")
		resultsDiv.Call("appendChild", h4)

		for _, m := range analysis.GoodMoves {
			div := doc.Call("createElement", "div")
			div.Get("classList").Call("add", "analysis-move")
			div.Get("classList").Call("add", "good")

			header := doc.Call("createElement", "div")
			header.Get("classList").Call("add", "move-header")
			header.Set("innerText", m.Move)
			div.Call("appendChild", header)

			exp := doc.Call("createElement", "div")
			exp.Get("classList").Call("add", "move-explanation")
			exp.Set("innerText", m.Explanation)
			div.Call("appendChild", exp)

			resultsDiv.Call("appendChild", div)
		}
	}

	// Bad Moves
	if len(analysis.BadMoves) > 0 {
		h4 := doc.Call("createElement", "h4")
		h4.Set("innerText", "🔴 Mistakes & Blunders")
		h4.Get("style").Set("color", "#f44336")
		h4.Get("style").Set("marginTop", "20px")
		resultsDiv.Call("appendChild", h4)

		for _, m := range analysis.BadMoves {
			div := doc.Call("createElement", "div")
			div.Get("classList").Call("add", "analysis-move")
			div.Get("classList").Call("add", "bad")

			header := doc.Call("createElement", "div")
			header.Get("classList").Call("add", "move-header")
			header.Set("innerText", m.Move)
			div.Call("appendChild", header)

			exp := doc.Call("createElement", "div")
			exp.Get("classList").Call("add", "move-explanation")
			exp.Set("innerText", m.Explanation)
			div.Call("appendChild", exp)

			resultsDiv.Call("appendChild", div)
		}
	}

	// Tips
	if len(analysis.Tips) > 0 {
		tipsDiv := doc.Call("createElement", "div")
		tipsDiv.Get("classList").Call("add", "tips-section")

		h4 := doc.Call("createElement", "h4")
		h4.Set("innerText", "🎓 Coach's Tips for Next Game")
		h4.Get("style").Set("margin", "0 0 10px 0")
		tipsDiv.Call("appendChild", h4)

		ul := doc.Call("createElement", "ul")
		ul.Get("style").Set("paddingLeft", "20px")
		ul.Get("style").Set("margin", "0")
		for _, tip := range analysis.Tips {
			li := doc.Call("createElement", "li")
			li.Set("innerText", tip)
			li.Get("style").Set("marginBottom", "5px")
			ul.Call("appendChild", li)
		}
		tipsDiv.Call("appendChild", ul)
		resultsDiv.Call("appendChild", tipsDiv)
	}
}
