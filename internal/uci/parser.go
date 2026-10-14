package uci

import (
	"strconv"
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
	HasScore bool
}

func ParseLine(line string) *Info {
	info := &Info{}
	parts := strings.Fields(line)

	if len(parts) >= 2 && parts[0] == "bestmove" {
		info.BestMove = parts[1]
		return info
	}

	for i := 0; i < len(parts)-1; i++ {
		switch parts[i] {
		case "depth":
			d, _ := strconv.Atoi(parts[i+1])
			info.Depth = d
		case "cp":
			c, _ := strconv.Atoi(parts[i+1])
			info.Score.CP = c
			info.HasScore = true
		case "mate":
			m, _ := strconv.Atoi(parts[i+1])
			info.Score.Mate = m
			info.Score.IsMate = true
			info.HasScore = true
		}
	}

	return info
}
