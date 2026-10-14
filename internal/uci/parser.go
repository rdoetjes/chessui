package uci

import (
	"fmt"
	"strings"
)

type Score struct {
	CP     int
	Mate   int
	IsMate bool
}

type Info struct {
	BestMove string
	Score    Score
	Depth    int
}

func ParseLine(line string) *Info {
	info := &Info{}
	parts := strings.Split(line, " ")

	if strings.HasPrefix(line, "bestmove") && len(parts) >= 2 {
		info.BestMove = parts[1]
		return info
	}

	for i, part := range parts {
		if part == "score" && i+2 < len(parts) {
			if parts[i+1] == "cp" {
				fmt.Sscanf(parts[i+2], "%d", &info.Score.CP)
			} else if parts[i+1] == "mate" {
				fmt.Sscanf(parts[i+2], "%d", &info.Score.Mate)
				info.Score.IsMate = true
			}
		}
		if part == "depth" && i+1 < len(parts) {
			fmt.Sscanf(parts[i+1], "%d", &info.Depth)
		}
	}

	return info
}
