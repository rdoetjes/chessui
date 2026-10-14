package chess_test

import (
	"testing"

	"github.com/notnil/chess"
)

func TestNewGame(t *testing.T) {
	game := chess.NewGame()
	if game.Position().String() != "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1" {
		t.Errorf("Unexpected initial FEN: %s", game.Position().String())
	}
}

func TestValidMove(t *testing.T) {
	game := chess.NewGame()
	// Move e2 to e4
	err := game.MoveStr("e2e4")
	if err != nil {
		t.Errorf("e2e4 should be a valid move: %v", err)
	}
}

func TestCheckmate(t *testing.T) {
	// Fool's mate
	game := chess.NewGame()
	game.MoveStr("f2f3")
	game.MoveStr("e7e5")
	game.MoveStr("g2g4")
	game.MoveStr("d8h4")

	if game.Outcome() != chess.BlackCheckmates {
		t.Errorf("Expected black to checkmate, got: %s", game.Outcome())
	}
}
