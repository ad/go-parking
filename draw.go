package main

import (
	"fmt"
	"image"
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

// DrawScaledAnalysisPoints отрисовывает точки анализа с масштабированием для оригинального изображения
func DrawScaledAnalysisPoints(imgGG *gg.Context, imgEdges *image.Gray, analysisPolygon *poly.Poly, lightColor, darkColor color.RGBA, threshold uint8, step int, scale float64) {
	bounds := imgEdges.Bounds()

	lightCount := 0
	darkCount := 0

	// Проходим по пикселям уменьшенного изображения с меньшим шагом для лучшей визуализации
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			point := poly.XY{X: float64(x), Y: float64(y)}
			if point.In(*analysisPolygon) {
				pixel := imgEdges.GrayAt(x, y)

				var pointColor color.RGBA
				var radius float64
				if pixel.Y >= threshold {
					pointColor = lightColor
					lightCount++
					radius = 2.0 // Светлые точки больше
				} else {
					pointColor = darkColor
					darkCount++
					radius = 2.0 // Темные точки меньше
				}

				// ИСПРАВЛЕНИЕ: Масштабируем координаты ВВЕРХ для отрисовки на оригинальном изображении
				// Если imgEdges уменьшено в scale раз, то для отрисовки на оригинале нужно умножить на scale
				scaledX := float64(x) * scale
				scaledY := float64(y) * scale

				// Рисуем точку с контрастной обводкой для лучшей видимости
				// imgGG.SetColor(color.RGBA{255, 255, 255, 255}) // Белая обводка
				// imgGG.DrawCircle(scaledX, scaledY, radius+1.5)
				// imgGG.Fill()

				imgGG.SetColor(pointColor)
				imgGG.DrawCircle(scaledX, scaledY, radius)
				imgGG.Fill()
			}
		}
	}

	// Выводим статистику для отладки
	fmt.Printf("  Debug points: light=%d, dark=%d, threshold=%d\n", lightCount, darkCount, threshold)
}
