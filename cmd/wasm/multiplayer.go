//go:build js && wasm

package main

import (
	"fmt"
	"strings"
)

// handleMultiplayerMessage processes relay commands from the remote player.
func handleMultiplayerMessage(msg string) bool {
	if !app.multi.isActive {
		return false
	}

	// 1. Peer move relay
	if strings.HasPrefix(msg, "opponent_move ") {
		moveStr := strings.TrimPrefix(msg, "opponent_move ")
		fmt.Printf("MULTIPLAYER: Opponent moved: %s\n", moveStr)
		executeMove(moveStr, false) // false = don't relay back to the sender
		return true
	}

	// 2. Color sync signals
	if strings.HasPrefix(msg, "assign_color ") {
		hostColor := strings.TrimPrefix(msg, "assign_color ")
		if hostColor == "white" {
			app.multi.playerColor = playerColorMap["black"]
		} else {
			app.multi.playerColor = playerColorMap["white"]
		}
		fmt.Printf("MULTIPLAYER: Auto-assigned color: %v\n", app.multi.playerColor)
		return true
	}

	return false
}

// relayMoveToOpponent sends your move to the server to be seen by the other player.
func relayMoveToOpponent(moveStr string) {
	if app.multi.isActive {
		sendUCI("move " + moveStr)
	}
}
