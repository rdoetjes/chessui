package main

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/notnil/chess"
	"github.com/rdoetjes/chess-ui/internal/uci"
)

var (
	game         *chess.Game
	boardCanvas  js.Value
	ctx          js.Value
	images       map[string]js.Value
	selectedPos  chess.Square = chess.NoSquare
	ws           js.Value
	currentStyle = "standard"
	isWhite      = true // Player color
	lastMove     *chess.Move
	evalScore    float64      = 0.0
	isGameOver                = false
	engineDepth               = 15
	isSetupMode               = false
	setupSquare  chess.Square = chess.NoSquare
	setupMap     map[chess.Square]chess.Piece
)

func main() {
	game = chess.NewGame()
	images = make(map[string]js.Value)
	setupMap = make(map[chess.Square]chess.Piece)

	fmt.Println("WASM: Initializing...")

	doc := js.Global().Get("document")
	boardCanvas = doc.Call("getElementById", "chess-board")
	if boardCanvas.IsNull() {
		fmt.Println("WASM Error: canvas not found")
		return
	}
	ctx = boardCanvas.Call("getContext", "2d")

	setupWebSocket()
	loadAssets()
	registerEvents()

	requestAnimationFrame := js.Global().Get("requestAnimationFrame")
	var render js.Func
	render = js.FuncOf(func(this js.Value, args []js.Value) any {
		drawBoard()
		requestAnimationFrame.Invoke(render)
		return nil
	})
	requestAnimationFrame.Invoke(render)

	select {}
}

func setupWebSocket() {
	loc := js.Global().Get("location")
	host := loc.Get("host").String()
	wsURL := "ws://" + host + "/ws"
	if loc.Get("protocol").String() == "https:" {
		wsURL = "wss://" + host + "/ws"
	}

	doc := js.Global().Get("document")
	infoPanel := doc.Call("getElementById", "engine-info")
	infoPanel.Set("innerText", "Connecting to engine...")

	ws = js.Global().Get("WebSocket").New(wsURL)

	ws.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println("WASM: WebSocket Connected")
		infoPanel.Set("innerText", "")
		sendUCI("uci")
		sendUCI("isready")
		return nil
	}))

	ws.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		handleUCIResponse(args[0].Get("data").String())
		return nil
	}))
}

func loadAssets() {
	pieces := []string{"wp", "wn", "wb", "wr", "wq", "wk", "bp", "bn", "bb", "br", "bq", "bk", "board"}
	for _, p := range pieces {
		img := js.Global().Get("Image").New()
		img.Set("src", fmt.Sprintf("/assets/pieces/%s/%s.png", currentStyle, p))
		images[p] = img
	}
}

func registerEvents() {
	doc := js.Global().Get("document")

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
		if len(moves) > 0 {
			game = chess.NewGame()
			for i := 0; i < len(moves)-1; i++ {
				game.Move(moves[i])
			}
			lastMove = nil
			if len(game.Moves()) > 0 {
				lastMove = game.Moves()[len(game.Moves())-1]
			}
			updateMoveList()
		}
		return nil
	}))

	doc.Call("getElementById", "setup-board").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		if !isSetupMode {
			isSetupMode = true
			this.Set("innerText", "Play from Position")
			doc.Call("getElementById", "setup-controls").Get("classList").Call("remove", "hidden")
			setupMap = make(map[chess.Square]chess.Piece)
			f, _ := chess.FEN("8/8/8/8/8/8/8/8 w - - 0 1")
			game = chess.NewGame(f)
		} else {
			whiteKing, blackKing := false, false
			for _, p := range setupMap {
				if p == chess.WhiteKing {
					whiteKing = true
				}
				if p == chess.BlackKing {
					blackKing = true
				}
			}
			if !whiteKing || !blackKing {
				js.Global().Call("alert", "Place both kings first!")
				return nil
			}
			isSetupMode = false
			this.Set("innerText", "Setup Board")
			doc.Call("getElementById", "setup-controls").Get("classList").Call("add", "hidden")
			turn := doc.Call("getElementById", "turn-select").Get("value").String()
			f, _ := chess.FEN(generateFEN(setupMap, turn))
			game = chess.NewGame(f)
			sendMoveToStockfish()
		}
		updateMoveList()
		return nil
	}))

	doc.Call("getElementById", "close-picker").Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) any {
		doc.Call("getElementById", "piece-picker").Get("classList").Call("add", "hidden")
		return nil
	}))

	initPiecePicker()
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
			if p == piece && sq != setupSquare {
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
	f, _ := chess.FEN(generateFEN(setupMap, "w"))
	game = chess.NewGame(f)
	js.Global().Get("document").Call("getElementById", "piece-picker").Get("classList").Call("add", "hidden")
}

func handleBoardClick(col, row int) {
	sq := chess.Square(row*8 + col)
	if isSetupMode {
		setupSquare = sq
		js.Global().Get("document").Call("getElementById", "piece-picker").Get("classList").Call("remove", "hidden")
		return
	}
	if selectedPos == chess.NoSquare {
		p := game.Position().Board().Piece(sq)
		if p != chess.NoPiece && p.Color() == game.Position().Turn() {
			selectedPos = sq
		}
	} else {
		for _, m := range game.ValidMoves() {
			if m.S1() == selectedPos && m.S2() == sq {
				game.Move(m)
				lastMove = m
				selectedPos = chess.NoSquare
				updateMoveList()
				sendMoveToStockfish()
				checkGameState()
				return
			}
		}
		p := game.Position().Board().Piece(sq)
		if p != chess.NoPiece && p.Color() == game.Position().Turn() {
			selectedPos = sq
		} else {
			selectedPos = chess.NoSquare
		}
	}
}

func handleUCIResponse(msg string) {
	info := uci.ParseLine(msg)
	if info.Score.IsMate || info.Score.CP != 0 {
		evalScore = float64(info.Score.CP) / 100.0
		if info.Score.IsMate {
			evalScore = 10.0
			if info.Score.Mate < 0 {
				evalScore = -10.0
			}
		}
		updateEvalBar()
	}
	if info.BestMove != "" && !isSetupMode && !isGameOver {
		for _, m := range game.ValidMoves() {
			if m.String() == info.BestMove {
				game.Move(m)
				lastMove = m
				updateMoveList()
				checkGameState()
				break
			}
		}
	}
}

func drawBoard() {
	rect := boardCanvas.Call("getBoundingClientRect")
	w := rect.Get("width").Float()
	if w < 10 {
		w = 600
	}
	if boardCanvas.Get("width").Float() != w {
		boardCanvas.Set("width", w)
		boardCanvas.Set("height", w)
	}
	size, tileSize := w, w/8
	ctx.Set("fillStyle", "#000")
	ctx.Call("fillRect", 0, 0, size, size)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if (x+y)%2 != 0 {
				ctx.Set("fillStyle", "#34495e")
			} else {
				ctx.Set("fillStyle", "#212f3c")
			}
			ctx.Call("fillRect", float64(x)*tileSize, float64(y)*tileSize, tileSize, tileSize)
		}
	}
	if isSetupMode {
		for sq, p := range setupMap {
			drawPieceAt(sq, p, tileSize)
		}
	} else {
		board := game.Position().Board()
		for i := 0; i < 64; i++ {
			sq := chess.Square(i)
			p := board.Piece(sq)
			if p != chess.NoPiece {
				drawPieceAt(sq, p, tileSize)
			}
		}
	}
	if lastMove != nil && !isSetupMode {
		ctx.Set("fillStyle", "rgba(255, 255, 0, 0.2)")
		ctx.Call("fillRect", float64(lastMove.S1().File())*tileSize, float64(7-lastMove.S1().Rank())*tileSize, tileSize, tileSize)
		ctx.Call("fillRect", float64(lastMove.S2().File())*tileSize, float64(7-lastMove.S2().Rank())*tileSize, tileSize, tileSize)
	}
}

func drawPieceAt(sq chess.Square, p chess.Piece, tileSize float64) {
	pStr, pChar := "w", ""
	if p.Color() == chess.Black {
		pStr = "b"
	}
	switch p.Type() {
	case chess.Pawn:
		pChar = "p"
	case chess.Knight:
		pChar = "n"
	case chess.Bishop:
		pChar = "b"
	case chess.Rook:
		pChar = "r"
	case chess.Queen:
		pChar = "q"
	case chess.King:
		pChar = "k"
	}
	pStr += pChar
	if img, ok := images[pStr]; ok && img.Get("complete").Bool() && img.Get("naturalWidth").Int() > 0 {
		ctx.Call("drawImage", img, float64(sq.File())*tileSize, float64(7-sq.Rank())*tileSize, tileSize, tileSize)
	} else {
		ctx.Set("fillStyle", "white")
		if p.Color() == chess.Black {
			ctx.Set("fillStyle", "#aaa")
		}
		ctx.Set("font", "bold 30px Arial")
		ctx.Call("fillText", pStr, float64(sq.File())*tileSize+tileSize/4, float64(7-sq.Rank())*tileSize+tileSize/1.5)
	}
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

func sendMoveToStockfish() {
	sendUCI("position fen " + game.Position().String())
	elo := js.Global().Get("document").Call("getElementById", "elo-select").Get("value").String()
	sendUCI("setoption name UCI_LimitStrength value true")
	sendUCI("setoption name UCI_Elo value " + elo)
	sendUCI(fmt.Sprintf("go depth %d", engineDepth))
}

func sendUCI(msg string) {
	if ws.Get("readyState").Int() == 1 {
		ws.Call("send", msg)
	}
}

func generateFEN(m map[chess.Square]chess.Piece, turn string) string {
	fen := ""
	for r := 7; r >= 0; r-- {
		empty := 0
		for f := 0; f < 8; f++ {
			p, ok := m[chess.Square(r*8+f)]
			if !ok {
				empty++
			} else {
				if empty > 0 {
					fen += fmt.Sprintf("%d", empty)
					empty = 0
				}
				char := "p"
				switch p.Type() {
				case chess.Knight:
					char = "n"
				case chess.Bishop:
					char = "b"
				case chess.Rook:
					char = "r"
				case chess.Queen:
					char = "q"
				case chess.King:
					char = "k"
				}
				if p.Color() == chess.White {
					char = strings.ToUpper(char)
				}
				fen += char
			}
		}
		if empty > 0 {
			fen += fmt.Sprintf("%d", empty)
		}
		if r > 0 {
			fen += "/"
		}
	}
	return fen + " " + turn + " - - 0 1"
}

func updateEvalBar() {
	doc := js.Global().Get("document")
	fill := doc.Call("getElementById", "eval-white-fill")
	wText, bText := doc.Call("getElementById", "eval-white-text"), doc.Call("getElementById", "eval-black-text")
	percent := 50.0 + (evalScore * 10.0)
	if percent > 100 {
		percent = 100
	} else if percent < 0 {
		percent = 0
	}
	fill.Get("style").Set("height", fmt.Sprintf("%f%%", percent))
	wText.Set("innerText", fmt.Sprintf("%.1f", evalScore))
	bText.Set("innerText", fmt.Sprintf("%.1f", -evalScore))
}

func checkGameState() {
	if game.Outcome() != chess.NoOutcome {
		isGameOver = true
		alert := js.Global().Get("document").Call("getElementById", "checkmate-alert")
		alert.Call("querySelector", "#winner-text").Set("innerText", "Outcome: "+game.Outcome().String())
		alert.Get("classList").Call("remove", "hidden")
	}
}
