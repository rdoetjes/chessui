package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	styles := []string{"standard", "classic", "modern"}
	for _, style := range styles {
		baseDir := filepath.Join("assets/pieces", style)
		os.MkdirAll(baseDir, 0755)

		pieces := []string{"wp", "wn", "wb", "wr", "wq", "wk", "bp", "bn", "bb", "br", "bq", "bk"}
		for _, p := range pieces {
			img := image.NewRGBA(image.Rect(0, 0, 128, 128))

			// Background transparent
			draw.Draw(img, img.Bounds(), &image.Uniform{color.Transparent}, image.Point{}, draw.Src)

			isWhite := p[0] == 'w'
			fillClr := color.RGBA{240, 240, 240, 255}
			strokeClr := color.RGBA{0, 0, 0, 255}
			if !isWhite {
				fillClr = color.RGBA{45, 45, 45, 255}
				strokeClr = color.RGBA{200, 200, 200, 255}
			}

			drawPiece(img, p[1], fillClr, strokeClr)

			f, _ := os.Create(filepath.Join(baseDir, p+".png"))
			png.Encode(f, img)
			f.Close()
		}

		// Generate board 4096x4096
		boardImg := image.NewRGBA(image.Rect(0, 0, 4096, 4096))
		light := color.RGBA{235, 235, 210, 255} // Cream
		dark := color.RGBA{115, 150, 85, 255}   // Greenish

		if style == "modern" {
			light = color.RGBA{240, 241, 243, 255}
			dark = color.RGBA{186, 202, 213, 255}
		} else if style == "classic" {
			light = color.RGBA{240, 217, 181, 255}
			dark = color.RGBA{181, 136, 99, 255}
		}

		tileSize := 4096 / 8
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				c := light
				if (x+y)%2 != 0 {
					c = dark
				}
				rect := image.Rect(x*tileSize, y*tileSize, (x+1)*tileSize, (y+1)*tileSize)
				draw.Draw(boardImg, rect, &image.Uniform{c}, image.Point{}, draw.Src)
			}
		}

		f, _ := os.Create(filepath.Join(baseDir, "board.png"))
		png.Encode(f, boardImg)
		f.Close()
	}
}

func drawPiece(img *image.RGBA, typ byte, fill, stroke color.Color) {
	switch typ {
	case 'p': // Pawn
		drawRect(img, 40, 100, 88, 115, fill) // Base
		drawCircle(img, 64, 45, 25, fill)     // Head
		drawRect(img, 54, 60, 74, 100, fill)  // Body
	case 'r': // Rook
		drawRect(img, 35, 95, 93, 115, fill) // Base
		drawRect(img, 45, 45, 83, 95, fill)  // Body
		drawRect(img, 35, 30, 93, 50, fill)  // Top
		// Crenelations
		drawRect(img, 35, 25, 45, 35, fill)
		drawRect(img, 59, 25, 69, 35, fill)
		drawRect(img, 83, 25, 93, 35, fill)
	case 'n': // Knight
		drawRect(img, 35, 95, 93, 115, fill) // Base
		drawRect(img, 45, 50, 75, 95, fill)  // Neck
		drawRect(img, 45, 35, 100, 65, fill) // Head
		drawRect(img, 45, 25, 60, 40, fill)  // Ear
	case 'b': // Bishop
		drawRect(img, 35, 95, 93, 115, fill) // Base
		drawCircle(img, 64, 55, 28, fill)    // Body/Head
		drawCircle(img, 64, 25, 8, fill)     // Tip
		drawRect(img, 54, 75, 74, 100, fill) // Neck
	case 'q': // Queen
		drawRect(img, 35, 95, 93, 115, fill) // Base
		drawRect(img, 50, 50, 78, 100, fill) // Body
		drawRect(img, 30, 35, 98, 55, fill)  // Crown base
		drawCircle(img, 35, 30, 8, fill)     // Point 1
		drawCircle(img, 64, 20, 8, fill)     // Point 2
		drawCircle(img, 93, 30, 8, fill)     // Point 3
	case 'k': // King
		drawRect(img, 35, 95, 93, 115, fill) // Base
		drawRect(img, 50, 50, 78, 100, fill) // Body
		drawRect(img, 35, 35, 93, 55, fill)  // Crown base
		drawRect(img, 60, 10, 68, 40, fill)  // Cross vertical
		drawRect(img, 50, 20, 78, 28, fill)  // Cross horizontal
	}
}

func drawRect(img *image.RGBA, x1, y1, x2, y2 int, c color.Color) {
	for y := y1; y < y2; y++ {
		for x := x1; x < x2; x++ {
			img.Set(x, y, c)
		}
	}
}

func drawCircle(img *image.RGBA, cx, cy, r int, c color.Color) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r {
				img.Set(x, y, c)
			}
		}
	}
}
