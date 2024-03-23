package main

import (
	"image/color"

	"github.com/ad/go-parking/poly"
	"github.com/fogleman/gg"
)

func DrawPolygon(imgGG *gg.Context, p *poly.Poly, col color.RGBA, lineWidth float64) {
	for i := 0; i < len(p.XY); i++ {
		a := p.XY[i]
		b := p.XY[(i+1)%len(p.XY)]

		imgGG.DrawLine(a.X, a.Y, b.X, b.Y)
		imgGG.SetColor(col)
		imgGG.SetLineWidth(lineWidth)
		imgGG.Stroke()
	}
}

func DrawStrokeText(imgGG *gg.Context, text string, x, y float64, textColor, strokeColor color.RGBA, strokeWidth float64) {
	imgGG.SetColor(strokeColor)
	for dy := -strokeWidth; dy <= strokeWidth; dy++ {
		for dx := -strokeWidth; dx <= strokeWidth; dx++ {
			if dx*dx+dy*dy >= strokeWidth*strokeWidth {
				// give it rounded corners
				continue
			}

			x := x + float64(dx)
			y := y + float64(dy)

			imgGG.DrawStringAnchored(text, x, y, 0.5, 0.5)
		}
	}

	imgGG.SetColor(textColor)
	imgGG.DrawStringAnchored(text, x, y, 0.5, 0.5)
}
