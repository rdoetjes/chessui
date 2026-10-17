//go:build js && wasm

package main

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/notnil/chess"
)

func generateUUID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[js.Global().Get("Math").Call("floor", js.Global().Get("Math").Call("random").Float()*float64(len(chars))).Int()]
	}
	return string(b)
}

func generateFEN(m map[chess.Square]chess.Piece, turn string) string {
	fen := ""
	for r := 7; r >= 0; r-- {
		empty := 0
		for f := 0; f < 8; f++ {
			sq := chess.Square(r*8 + f)
			p, ok := m[sq]
			if !ok {
				empty++
			} else {
				if empty > 0 {
					fen += fmt.Sprintf("%d", empty)
					empty = 0
				}
				char := ""
				switch p.Type() {
				case chess.Pawn:
					char = "p"
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
	fen += fmt.Sprintf(" %s - - 0 1", turn)
	return fen
}

func setCookie(name, value string, days int) {
	doc := js.Global().Get("document")
	expires := ""
	if days > 0 {
		date := js.Global().Get("Date").New()
		time := date.Call("getTime").Float() + float64(days*24*60*60*1000)
		date.Call("setTime", time)
		expires = "; expires=" + date.Call("toUTCString").String()
	}
	doc.Set("cookie", name+"="+value+expires+"; path=/")
}

func getCookie(name string) string {
	doc := js.Global().Get("document")
	cookieStr := doc.Get("cookie").String()
	cookies := strings.Split(cookieStr, "; ")
	for _, cookie := range cookies {
		parts := strings.Split(cookie, "=")
		if len(parts) == 2 && parts[0] == name {
			return parts[1]
		}
	}
	return ""
}
