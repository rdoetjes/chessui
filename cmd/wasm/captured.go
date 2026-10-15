//go:build js && wasm

package main

import (
	"fmt"
	"sort"
	"syscall/js"

	"github.com/notnil/chess"
)

var pieceValues = map[chess.PieceType]int{
	chess.Pawn:   1,
	chess.Knight: 3,
	chess.Bishop: 3,
	chess.Rook:   5,
	chess.Queen:  9,
	chess.King:   0,
}

func updateCapturedPieces() {
	doc := js.Global().Get("document")
	topContainer := doc.Call("getElementById", "captured-top")
	bottomContainer := doc.Call("getElementById", "captured-bottom")

	if topContainer.IsNull() || bottomContainer.IsNull() {
		return
	}

	topPiecesDiv := topContainer.Call("querySelector", ".captured-pieces")
	topScoreDiv := topContainer.Call("querySelector", ".captured-score")
	bottomPiecesDiv := bottomContainer.Call("querySelector", ".captured-pieces")
	bottomScoreDiv := bottomContainer.Call("querySelector", ".captured-score")

	topPiecesDiv.Set("innerHTML", "")
	bottomPiecesDiv.Set("innerHTML", "")

	// Count pieces currently on board
	currentPieces := make(map[chess.Piece]int)
	board := game.Position().Board()
	for i := 0; i < 64; i++ {
		p := board.Piece(chess.Square(i))
		if p != chess.NoPiece {
			currentPieces[p]++
		}
	}

	// Starting pieces
	startingPieces := []struct {
		p     chess.Piece
		count int
	}{
		{chess.WhitePawn, 8}, {chess.WhiteKnight, 2}, {chess.WhiteBishop, 2}, {chess.WhiteRook, 2}, {chess.WhiteQueen, 1},
		{chess.BlackPawn, 8}, {chess.BlackKnight, 2}, {chess.BlackBishop, 2}, {chess.BlackRook, 2}, {chess.BlackQueen, 1},
	}

	whitePiecesTakenByBlack := []chess.Piece{}
	blackPiecesTakenByWhite := []chess.Piece{}
	whiteValueTakenByBlack := 0
	blackValueTakenByWhite := 0

	for _, sp := range startingPieces {
		capturedCount := sp.count - currentPieces[sp.p]
		for i := 0; i < capturedCount; i++ {
			if sp.p.Color() == chess.White {
				whitePiecesTakenByBlack = append(whitePiecesTakenByBlack, sp.p)
				whiteValueTakenByBlack += pieceValues[sp.p.Type()]
			} else {
				blackPiecesTakenByWhite = append(blackPiecesTakenByWhite, sp.p)
				blackValueTakenByWhite += pieceValues[sp.p.Type()]
			}
		}
	}

	sortPieces(whitePiecesTakenByBlack)
	sortPieces(blackPiecesTakenByWhite)

	// Who is at the bottom? playerColor.
	// Who is at the top? Opponent.
	if playerColor == chess.White {
		// Bottom is White. Show pieces White has taken (Black pieces).
		renderCaptured(bottomPiecesDiv, bottomScoreDiv, blackPiecesTakenByWhite, blackValueTakenByWhite)
		// Top is Black. Show pieces Black has taken (White pieces).
		renderCaptured(topPiecesDiv, topScoreDiv, whitePiecesTakenByBlack, whiteValueTakenByBlack)
	} else {
		// Bottom is Black. Show pieces Black has taken (White pieces).
		renderCaptured(bottomPiecesDiv, bottomScoreDiv, whitePiecesTakenByBlack, whiteValueTakenByBlack)
		// Top is White. Show pieces White has taken (Black pieces).
		renderCaptured(topPiecesDiv, topScoreDiv, blackPiecesTakenByWhite, blackValueTakenByWhite)
	}
}

func sortPieces(pieces []chess.Piece) {
	sort.Slice(pieces, func(i, j int) bool {
		return pieceValues[pieces[i].Type()] < pieceValues[pieces[j].Type()]
	})
}

func renderCaptured(piecesDiv, scoreDiv js.Value, pieces []chess.Piece, totalValue int) {
	doc := js.Global().Get("document")
	for _, p := range pieces {
		pStr := "w"
		if p.Color() == chess.Black {
			pStr = "b"
		}
		switch p.Type() {
		case chess.Pawn:
			pStr += "p"
		case chess.Knight:
			pStr += "n"
		case chess.Bishop:
			pStr += "b"
		case chess.Rook:
			pStr += "r"
		case chess.Queen:
			pStr += "q"
		case chess.King:
			pStr += "k"
		}

		img := doc.Call("createElement", "img")
		# Use relative path (no leading slash)
		img.Set("src", fmt.Sprintf("assets/pieces/%s/%s.png", currentStyle, pStr))
		img.Get("style").Set("width", "24px")
		img.Get("style").Set("height", "24px")
		piecesDiv.Call("appendChild", img)
	}

	if len(pieces) > 0 {
		scoreDiv.Set("innerText", fmt.Sprintf("+%d", totalValue))
		scoreDiv.Get("style").Set("display", "inline-block")
	} else {
		scoreDiv.Set("innerText", "")
		scoreDiv.Get("style").Set("display", "none")
	}
}
