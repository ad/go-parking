package main

import (
	"image"
	"image/color"
	"math"

	"github.com/ad/go-parking/poly"
	"github.com/ernyoke/imger/blur"
	"github.com/ernyoke/imger/effects"
	"github.com/ernyoke/imger/grayscale"
	"github.com/ernyoke/imger/padding"
	"github.com/ernyoke/imger/resize"
)

// ImageProcessor содержит улучшенные алгоритмы обработки изображений
type ImageProcessor struct {
	// Параметры адаптивной обработки
	AdaptiveThreshold bool
	NoiseReduction    bool
	MultiScale        bool

	// Статистики для автокалибровки
	HistogramStats *ImageStats
}

// ImageStats содержит статистики изображения для адаптивной обработки
type ImageStats struct {
	MeanBrightness float64
	Contrast       float64
	NoiseLevel     float64
	IsLowLight     bool
	IsHighContrast bool
}

// ProcessingParams содержит параметры обработки
type ProcessingParams struct {
	IsDay           bool
	ThresholdEmpty  float64
	ThresholdEdges  float64
	ResizeScale     float64
	DenoiseStrength float64
	ContrastBoost   float64
	PolygonScale    float64
	OffsetX         float64
	OffsetY         float64
}

// NewImageProcessor создает новый процессор изображений
func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		AdaptiveThreshold: true,
		NoiseReduction:    true,
		MultiScale:        true,
	}
}

// ProcessImageAdvanced выполняет улучшенную обработку изображения
func (ip *ImageProcessor) ProcessImageAdvanced(img image.Image, params ProcessingParams) (image.Image, *ImageStats, error) {
	// 1. Анализ изображения для получения статистик
	stats := ip.analyzeImage(img)
	ip.HistogramStats = stats

	// 2. Адаптивная коррекция параметров
	adaptedParams := ip.adaptParameters(params, stats)

	// 3. Конвертация в grayscale
	grayscaleImg := grayscale.Grayscale(img)

	// 4. Адаптивная коррекция освещения
	if stats.IsLowLight {
		grayscaleImg = ip.enhanceLowLight(grayscaleImg)
	}

	// 5. Шумоподавление
	if ip.NoiseReduction {
		grayscaleImg = ip.reduceNoise(grayscaleImg, adaptedParams.DenoiseStrength)
	}

	// 6. Адаптивное улучшение контраста
	grayscaleImg = ip.adaptiveContrastEnhancement(grayscaleImg, stats)

	// 7. Повышение резкости
	sharpened, err := effects.SharpenGray(grayscaleImg)
	if err != nil {
		return nil, stats, err
	}

	// 8. Масштабирование
	if adaptedParams.ResizeScale != 1.0 {
		sharpened, err = resize.ResizeGray(sharpened, adaptedParams.ResizeScale, adaptedParams.ResizeScale, resize.InterNearest)
		if err != nil {
			return nil, stats, err
		}
	}

	return sharpened, stats, nil
}

// analyzeImage анализирует изображение для получения статистик
func (ip *ImageProcessor) analyzeImage(img image.Image) *ImageStats {
	bounds := img.Bounds()
	totalPixels := bounds.Dx() * bounds.Dy()

	var brightnesSum float64
	var histogram [256]int

	// Проходим по всем пикселям
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()

			// Конвертируем в grayscale
			gray := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256
			grayInt := int(gray)
			if grayInt > 255 {
				grayInt = 255
			}

			histogram[grayInt]++
			brightnesSum += gray
		}
	}

	// Рассчитываем среднюю яркость
	meanBrightness := brightnesSum / float64(totalPixels)

	// Рассчитываем контраст (стандартное отклонение)
	var variance float64
	for i, count := range histogram {
		if count > 0 {
			diff := float64(i) - meanBrightness
			variance += diff * diff * float64(count)
		}
	}
	contrast := math.Sqrt(variance / float64(totalPixels))

	// Определяем уровень шума (упрощенная оценка)
	noiseLevel := ip.estimateNoiseLevel(histogram)

	return &ImageStats{
		MeanBrightness: meanBrightness,
		Contrast:       contrast,
		NoiseLevel:     noiseLevel,
		IsLowLight:     meanBrightness < 80,
		IsHighContrast: contrast > 50,
	}
}

// adaptParameters адаптирует параметры обработки на основе статистик
func (ip *ImageProcessor) adaptParameters(params ProcessingParams, stats *ImageStats) ProcessingParams {
	adapted := params

	// Адаптация порогов на основе яркости
	if stats.IsLowLight {
		adapted.ThresholdEmpty *= 0.85 // Снижаем порог для темных изображений
		adapted.ThresholdEdges *= 0.9
		adapted.ContrastBoost = 1.3
	} else if stats.MeanBrightness > 180 {
		adapted.ThresholdEmpty *= 1.1 // Повышаем порог для ярких изображений
		adapted.ThresholdEdges *= 1.1
		adapted.ContrastBoost = 0.9
	}

	// Адаптация шумоподавления
	if stats.NoiseLevel > 0.1 {
		adapted.DenoiseStrength *= 1.5
	}

	return adapted
}

// enhanceLowLight улучшает изображения при слабом освещении
func (ip *ImageProcessor) enhanceLowLight(img *image.Gray) *image.Gray {
	bounds := img.Bounds()
	enhanced := image.NewGray(bounds)

	// Применяем гамма-коррекцию для улучшения темных областей
	gamma := 0.7

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := img.GrayAt(x, y)

			// Нормализуем в диапазон 0-1
			normalized := float64(pixel.Y) / 255.0

			// Применяем гамма-коррекцию
			corrected := math.Pow(normalized, gamma)

			// Обратно в диапазон 0-255
			newValue := uint8(corrected * 255)

			enhanced.SetGray(x, y, color.Gray{Y: newValue})
		}
	}

	return enhanced
}

// reduceNoise выполняет шумоподавление
func (ip *ImageProcessor) reduceNoise(img *image.Gray, strength float64) *image.Gray {
	// Используем Gaussian blur для шумоподавления
	sigma := strength
	blurred, err := blur.GaussianBlurGray(img, sigma, sigma, padding.BorderReflect)
	if err != nil {
		return img // Возвращаем исходное изображение в случае ошибки
	}

	return blurred
}

// adaptiveContrastEnhancement выполняет адаптивное улучшение контраста
func (ip *ImageProcessor) adaptiveContrastEnhancement(img *image.Gray, stats *ImageStats) *image.Gray {
	bounds := img.Bounds()
	enhanced := image.NewGray(bounds)

	// Параметры для адаптивного улучшения контраста
	alpha := 1.0 // Коэффициент контраста
	beta := 0.0  // Коэффициент яркости

	// Адаптируем параметры на основе статистик
	if stats.IsLowLight {
		alpha = 1.3
		beta = 20
	} else if stats.Contrast < 30 {
		alpha = 1.2
		beta = 10
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := img.GrayAt(x, y)

			// Применяем линейную коррекцию
			newValue := alpha*float64(pixel.Y) + beta

			// Ограничиваем значения диапазоном 0-255
			if newValue > 255 {
				newValue = 255
			} else if newValue < 0 {
				newValue = 0
			}

			enhanced.SetGray(x, y, color.Gray{Y: uint8(newValue)})
		}
	}

	return enhanced
}

// estimateNoiseLevel оценивает уровень шума в изображении
func (ip *ImageProcessor) estimateNoiseLevel(histogram [256]int) float64 {
	// Упрощенная оценка шума на основе гистограммы
	// Шум характеризуется наличием множества пиков малой амплитуды

	var peaks int
	threshold := 100 // Минимальное количество пикселей для считывания пика

	for i := 1; i < 255; i++ {
		if histogram[i] > threshold {
			if histogram[i] > histogram[i-1] && histogram[i] > histogram[i+1] {
				peaks++
			}
		}
	}

	// Нормализуем количество пиков
	noiseLevel := float64(peaks) / 20.0
	if noiseLevel > 1.0 {
		noiseLevel = 1.0
	}

	return noiseLevel
}

// MultiScaleAnalysis выполняет анализ на разных масштабах
func (ip *ImageProcessor) MultiScaleAnalysis(img *image.Gray, polygon *poly.Poly, params ProcessingParams) []float64 {
	scales := []float64{0.8, 1.0, 1.2}
	results := make([]float64, len(scales))

	for i, scale := range scales {
		// Анализируем полигон на исходном изображении с разными фильтрами
		results[i] = ip.analyzePolygonAtScale(img, polygon, scale, params)
	}

	return results
}

// analyzePolygonAtScale анализирует полигон с разными параметрами обработки
func (ip *ImageProcessor) analyzePolygonAtScale(img *image.Gray, polygon *poly.Poly, sensitivity float64, params ProcessingParams) float64 {
	zero, nonZero := 0, 0
	totalPixels := 0

	bounds := img.Bounds()

	// Применяем чувствительность к базовому порогу (128 - средний уровень серого)
	// sensitivity: 0.8 = более строгий (102), 1.0 = нормальный (128), 1.2 = более мягкий (154)
	baseThreshold := 96.0
	threshold := uint8(baseThreshold * sensitivity)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Преобразуем координаты пикселя к координатам полигона
			// Учитываем, что изображение было уменьшено на params.ResizeScale
			px := float64(x) / params.ResizeScale
			py := float64(y) / params.ResizeScale

			point := poly.XY{X: px, Y: py}
			if point.In(*polygon) {
				totalPixels++
				pixel := img.GrayAt(x, y)

				// Используем чувствительность для анализа
				if pixel.Y > threshold {
					zero++ // Светлый пиксель (возможно свободное место)
				} else {
					nonZero++ // Темный пиксель (возможно занято)
				}
			}
		}
	}

	if totalPixels == 0 {
		return -1 // Полигон не пересекается с изображением
	}

	// Возвращаем процент свободного места
	return float64(zero) / float64(totalPixels) * 100
}

// GetWeightedResult возвращает взвешенный результат многомасштабного анализа
func (ip *ImageProcessor) GetWeightedResult(results []float64) float64 {
	if len(results) == 0 {
		return 0
	}

	// Веса для разных масштабов (больший вес для основного масштаба)
	weights := []float64{0.3, 0.5, 0.2}

	var weightedSum, totalWeight float64

	for i, result := range results {
		if result >= 0 && i < len(weights) {
			weightedSum += result * weights[i]
			totalWeight += weights[i]
		}
	}

	if totalWeight == 0 {
		return 0
	}

	return weightedSum / totalWeight
}
