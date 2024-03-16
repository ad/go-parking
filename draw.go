package main

import (
	"image"
	"image/color"
	"math"
	"math/rand"

	"github.com/ad/go-parking/poly"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func DrawPolygon(img *image.RGBA, p *poly.Poly, col color.RGBA) {
	for i := 0; i < len(p.XY); i++ {
		a := p.XY[i]
		b := p.XY[(i+1)%len(p.XY)]

		DrawLine(image.Point{int(a.X), int(a.Y)}, image.Point{int(b.X), int(b.Y)}, func(p image.Point) {
			// img.Set(p.X, p.Y, col)
			img.Set(p.X+random(-1, 1), p.Y+random(-1, 1), col)
			img.Set(p.X+random(-1, 1), p.Y+random(-1, 1), col)
		})
	}
}

func random(min, max int) int {
	return rand.Intn(max-min+1) + min
}

func DrawLabel(img *image.RGBA, x, y int, label string, col color.RGBA) {
	point := fixed.Point26_6{
		X: fixed.I(x - 14),
		Y: fixed.I(y + 7),
	}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  point,
	}

	d.DrawString(label)
}

// DrawLine takes 2 image.Point (start & end) and a callback function given line points.
func DrawLine(a, b image.Point, point func(image.Point)) {
	steep := math.Abs(float64(b.Y-a.Y)) > math.Abs(float64(b.X-a.X))
	if steep {
		a.X, a.Y = a.Y, a.X
		b.X, b.Y = b.Y, b.X
	}
	if a.X > b.X {
		a.X, b.X = b.X, a.X
		a.Y, b.Y = b.Y, a.Y
	}

	dX := b.X - a.X
	dY := int(math.Abs(float64(b.Y - a.Y)))
	er := dX / 2

	y := a.Y
	var yStep int
	if a.Y < b.Y {
		yStep = 1
	} else {
		yStep = -1
	}

	for x := a.X; x <= b.X; x++ {
		var pos image.Point
		if steep {
			pos = image.Pt(y, x)
		} else {
			pos = image.Pt(x, y)
		}

		point(pos)

		er = er - dY
		if er < 0 {
			y = y + yStep
			er = er + dX
		}
	}
}
