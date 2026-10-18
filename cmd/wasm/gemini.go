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
		app.game.isGameOver = false
		doc.Call("getElementById", "popup-analysis").Get("classList").Call("remove", "hidden")

		key := getCookie("gemini-api-key")
		if key != "" {
			doc.Call("getElementById", "gemini-key-inline").Set("value", key)
			// Try to load models automatically if we have a key
			go refreshModels(key)
		}
		return nil
	}))

	doc.Call("getElementById", "btn-refresh-models").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		key := doc.Call("getElementById", "gemini-key-inline").Get("value").String()
		if key == "" {
			js.Global().Call("alert", "Please enter your Gemini API Key first.")
			return nil
		}
		go refreshModels(key)
		return nil
	}))

	doc.Call("getElementById", "btn-start-analysis").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		key := doc.Call("getElementById", "gemini-key-inline").Get("value").String()
		model := doc.Call("getElementById", "gemini-model-select").Get("value").String()

		if key == "" {
			js.Global().Call("alert", "Please enter your Gemini API Key first.")
			return nil
		}
		setCookie("gemini-api-key", key, 365)
		setCookie("gemini-model", model, 365)
		startAnalysis(key, model)
		return nil
	}))

	doc.Call("getElementById", "btn-analysis-close").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		doc.Call("getElementById", "popup-analysis").Get("classList").Call("add", "hidden")
		return nil
	}))

	makeDraggable(doc.Call("getElementById", "popup-analysis"), doc.Call("getElementById", "analysis-header"))
}

func makeDraggable(el js.Value, header js.Value) {
	var pos1, pos2, pos3, pos4 int

	var onMouseMove js.Func
	var onMouseUp js.Func

	onMouseMove = js.FuncOf(func(this js.Value, args []js.Value) any {
		e := args[0]
		pos1 = pos3 - e.Get("clientX").Int()
		pos2 = pos4 - e.Get("clientY").Int()
		pos3 = e.Get("clientX").Int()
		pos4 = e.Get("clientY").Int()

		el.Get("style").Set("top", fmt.Sprintf("%dpx", el.Get("offsetTop").Int()-pos2))
		el.Get("style").Set("left", fmt.Sprintf("%dpx", el.Get("offsetLeft").Int()-pos1))
		return nil
	})

	onMouseUp = js.FuncOf(func(this js.Value, args []js.Value) any {
		js.Global().Get("document").Call("removeEventListener", "mousemove", onMouseMove)
		js.Global().Get("document").Call("removeEventListener", "mouseup", onMouseUp)
		// We don't release here because we might drag again, but in a more complex app you might manage lifecycle
		return nil
	})

	header.Set("onmousedown", js.FuncOf(func(this js.Value, args []js.Value) any {
		e := args[0]
		e.Call("preventDefault")
		pos3 = e.Get("clientX").Int()
		pos4 = e.Get("clientY").Int()

		js.Global().Get("document").Call("addEventListener", "mousemove", onMouseMove)
		js.Global().Get("document").Call("addEventListener", "mouseup", onMouseUp)
		return nil
	}))
}

func refreshModels(apiKey string) {
	doc := js.Global().Get("document")
	statusDiv := doc.Call("getElementById", "model-status")
	selectEl := doc.Call("getElementById", "gemini-model-select")
	savedModel := getCookie("gemini-model")

	statusDiv.Set("innerText", "Loading models...")

	url := "https://generativelanguage.googleapis.com/v1beta/models?key=" + apiKey
	promise := js.Global().Call("fetch", url)

	ch := make(chan js.Value)
	promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- args[0]
		return nil
	})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- args[0]
		return nil
	}))

	resp := <-ch
	if resp.Type() != js.TypeObject || !resp.Get("ok").Bool() {
		statusDiv.Set("innerText", "Failed to load models. Check your API key.")
		return
	}

	textCh := make(chan string)
	resp.Call("text").Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
		textCh <- args[0].String()
		return nil
	}))

	bodyText := <-textCh

	// Debug: You can see the raw response in the console
	js.Global().Get("console").Call("log", "Gemini Models Response:", bodyText)

	var modelsResp struct {
		Models []struct {
			Name             string   `json:"name"`
			DisplayName      string   `json:"displayName"`
			SupportedMethods []string `json:"supportedMethods"`
		} `json:"models"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(bodyText), &modelsResp); err != nil {
		statusDiv.Set("innerText", "Error parsing models: "+err.Error())
		return
	}

	if modelsResp.Error.Message != "" {
		statusDiv.Set("innerText", "API Error: "+modelsResp.Error.Message)
		return
	}

	// Clear select
	selectEl.Set("innerHTML", "")

	count := 0
	for _, m := range modelsResp.Models {
		// Name is usually "models/gemini-..."
		shortName := m.Name
		if len(shortName) > 7 && shortName[:7] == "models/" {
			shortName = shortName[7:]
		}

		// Skip non-gemini models for brevity, but keep them if we find nothing else
		isGemini := (len(shortName) >= 6 && shortName[:6] == "gemini")

		canGenerate := false
		for _, method := range m.SupportedMethods {
			if method == "generateContent" {
				canGenerate = true
				break
			}
		}

		if canGenerate || isGemini {
			option := doc.Call("createElement", "option")
			option.Set("value", shortName)
			displayName := m.DisplayName
			if displayName == "" {
				displayName = shortName
			}
			option.Set("innerText", displayName)

			if shortName == savedModel || (savedModel == "" && shortName == "gemini-1.5-flash") {
				option.Set("selected", true)
			}
			selectEl.Call("appendChild", option)
			count++
		}
	}

	if count == 0 {
		// Fallback if no models found via API
		statusDiv.Set("innerText", "No models found in API. Adding defaults.")
		for _, def := range []string{"gemini-2.0-flash", "gemini-1.5-flash", "gemini-1.5-pro"} {
			option := doc.Call("createElement", "option")
			option.Set("value", def)
			option.Set("innerText", def)
			if def == "gemini-1.5-flash" {
				option.Set("selected", true)
			}
			selectEl.Call("appendChild", option)
		}
	} else {
		statusDiv.Set("innerText", fmt.Sprintf("Found %d models. Select one and click 'Start Analysis'.", count))
	}
}

func startAnalysis(apiKey, model string) {
	doc := js.Global().Get("document")

	doc.Call("getElementById", "analysis-status").Get("classList").Call("remove", "hidden")
	doc.Call("getElementById", "analysis-status").Set("innerText", fmt.Sprintf("Analyzing game with %s...", model))
	doc.Call("getElementById", "analysis-results").Get("classList").Call("add", "hidden")
	doc.Call("getElementById", "analysis-results").Set("innerHTML", "")

	pgn := fmt.Sprintf("[Event \"Casual Game\"]\n[Site \"chess.phonax.com\"]\n[Date \"??.??.??\"]\n[White \"Player\"]\n[Black \"Bot\"]\n[Result \"*\"]\n\n%s", app.game.game.String())

	go func() {
		analysis, err := fetchGeminiAnalysis(apiKey, model, pgn)
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

func fetchGeminiAnalysis(apiKey, model, pgn string) (*GeminiAnalysis, error) {
	prompt := fmt.Sprintf(`You are a professional chess coach. Analyze the following chess game in PGN format. Identify 2-3 key good moves and 2-3 key bad moves (mistakes or blunders). For each, explain simply why it was good or bad. Finally, give 3 concise tips for the player to improve in their next game.

PGN:
%s

Format your response ONLY as a JSON object with the following structure:
{
  "good_moves": [{"move": "move in algebraic notation", "explanation": "short explanation"}],
  "bad_moves": [{"move": "move in algebraic notation", "explanation": "short explanation"}],
  "tips": ["tip 1", "tip 2", "tip 3"]
}`, pgn)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)

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
	if resp.Type() != js.TypeObject {
		return nil, fmt.Errorf("failed to reach Gemini API: unexpected response type")
	}

	// Read body
	textPromise := resp.Call("text")
	textCh := make(chan string)
	textPromise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
		textCh <- args[0].String()
		return nil
	}))

	bodyText := <-textCh
	status := resp.Get("status").Int()

	if status < 200 || status >= 300 {
		var geminiErr struct {
			Error struct {
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		json.Unmarshal([]byte(bodyText), &geminiErr)

		errMsg := geminiErr.Error.Message
		if errMsg == "" {
			errMsg = bodyText
		}
		if errMsg == "" {
			errMsg = resp.Get("statusText").String()
		}

		if status == 404 {
			return nil, fmt.Errorf("Gemini API Error 404: Model '%s' not found or not available for your account. (Details: %s)", model, errMsg)
		}
		return nil, fmt.Errorf("Gemini API Error %d: %s", status, errMsg)
	}

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
		return nil, fmt.Errorf("%s", geminiResp.Error.Message)
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
