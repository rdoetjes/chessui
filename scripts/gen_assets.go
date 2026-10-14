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
			draw.Draw(img, img.Bounds(), &image.Uniform{color.Transparent}, image.Point{}, draw.Src)

			isWhite := p[0] == 'w'
			fillClr := color.RGBA{255, 255, 255, 255}
			borderClr := color.RGBA{0, 0, 0, 255}
			if !isWhite {
				fillClr = color.RGBA{40, 40, 40, 255}
				borderClr = color.RGBA{220, 220, 220, 255}
			}

			// Draw piece with border
			drawPieceWithBorder(img, p[1], fillClr, borderClr)

			f, _ := os.Create(filepath.Join(baseDir, p+".png"))
			png.Encode(f, img)
			f.Close()
		}

		// Generate board 4096x4096 - Neutral High Contrast
		boardImg := image.NewRGBA(image.Rect(0, 0, 4096, 4096))
		light := color.RGBA{230, 230, 230, 255} // Light Grey
		dark := color.RGBA{100, 100, 100, 255}  // Medium Grey

		if style == "standard" {
			light = color.RGBA{235, 236, 208, 255} // Classic Cream
			dark = color.RGBA{119, 149, 86, 255}   // Classic Green
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

func drawPieceWithBorder(img *image.RGBA, typ byte, fill, border color.Color) {
	// Simple border effect by drawing larger primitives first
	drawPiece(img, typ, border, 3) // Border
	drawPiece(img, typ, fill, 0)   // Interior
}

func drawPiece(img *image.RGBA, typ byte, c color.Color, offset int) {
	switch typ {
	case 'p':
		drawRect(img, 40-offset, 100-offset, 88+offset, 115+offset, c)
		drawCircle(img, 64, 45, 25+offset, c)
		drawRect(img, 54-offset, 60-offset, 74+offset, 100+offset, c)
	case 'r':
		drawRect(img, 35-offset, 95-offset, 93+offset, 115+offset, c)
		drawRect(img, 45-offset, 45-offset, 83+offset, 95+offset, c)
		drawRect(img, 35-offset, 30-offset, 93+offset, 50+offset, c)
	case 'n':
		drawRect(img, 35-offset, 95-offset, 93+offset, 115+offset, c)
		drawRect(img, 45-offset, 50-offset, 75+offset, 95+offset, c)
		drawRect(img, 45-offset, 35-offset, 100+offset, 65+offset, c)
	case 'b':
		drawRect(img, 35-offset, 95-offset, 93+offset, 115+offset, c)
		drawCircle(img, 64, 55, 28+offset, c)
		drawCircle(img, 64, 25, 8+offset, c)
	case 'q':
		drawRect(img, 35-offset, 95-offset, 93+offset, 115+offset, c)
		drawRect(img, 50-offset, 50-offset, 78+offset, 100+offset, c)
		drawCircle(img, 64, 30, 20+offset, c)
	case 'k':
		drawRect(img, 35-offset, 95-offset, 93+offset, 115+offset, c)
		drawRect(img, 50-offset, 50-offset, 78+offset, 100+offset, c)
		drawRect(img, 60-offset, 10-offset, 68+offset, 40+offset, c)
		drawRect(img, 50-offset, 20-offset, 78+offset, 28+offset, c)
	}
}

func drawRect(img *image.RGBA, x1, y1, x2, y2 int, c color.Color) {
	for y := y1; y < y2; y++ {
		for x := x1; x < x2; x++ {
			if x >= 0 && x < 128 && y >= 0 && y < 128 {
				img.Set(x, y, c)
			}
		}
	}
}

func drawCircle(img *image.RGBA, cx, cy, r int, c color.Color) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r {
				if x >= 0 && x < 128 && y >= 0 && y < 128 {
					img.Set(x, y, c)
				}
			}
		}
	}
}
