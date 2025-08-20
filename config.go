package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ad/go-parking/poly"
)

// Config содержит настройки приложения
type Config struct {
	// Основные настройки
	Port    int    `json:"port"`
	Version string `json:"version"`

	// Настройки обработки изображений
	Processing ProcessingConfig `json:"processing"`

	// Настройки адаптивности
	Adaptive AdaptiveConfig `json:"adaptive"`

	// Настройки Telegram
	Telegram TelegramConfig `json:"telegram"`

	// Полигоны парковочных мест
	Polygons PolygonConfig `json:"polygons"`
}

// ProcessingConfig содержит параметры обработки
type ProcessingConfig struct {
	DefaultResizeScale     float64 `json:"default_resize_scale"`
	DefaultDenoiseStrength float64 `json:"default_denoise_strength"`
	DefaultContrastBoost   float64 `json:"default_contrast_boost"`

	// Пороги для дня
	DayThresholdEmpty float64 `json:"day_threshold_empty"`
	DayThresholdEdges float64 `json:"day_threshold_edges"`

	// Пороги для ночи
	NightThresholdEmpty float64 `json:"night_threshold_empty"`
	NightThresholdEdges float64 `json:"night_threshold_edges"`

	// Включение улучшенных алгоритмов
	EnableAdvancedProcessing bool `json:"enable_advanced_processing"`
	EnableMultiScale         bool `json:"enable_multi_scale"`
	EnableAdaptiveThreshold  bool `json:"enable_adaptive_threshold"`
	EnableNoiseReduction     bool `json:"enable_noise_reduction"`
	EnableDebugVisualization bool `json:"enable_debug_visualization"`

	// Параметры трансформации полигонов
	PolygonScale float64 `json:"polygon_scale"`
	OffsetX      float64 `json:"offset_x"`
	OffsetY      float64 `json:"offset_y"`
}

// AdaptiveConfig содержит настройки адаптивности
type AdaptiveConfig struct {
	// Пороги для определения условий
	LowLightThreshold     float64 `json:"low_light_threshold"`
	HighContrastThreshold float64 `json:"high_contrast_threshold"`
	NoiseThreshold        float64 `json:"noise_threshold"`

	// Коэффициенты адаптации
	LowLightAdaptationFactor float64 `json:"low_light_adaptation_factor"`
	BrightAdaptationFactor   float64 `json:"bright_adaptation_factor"`
	NoiseAdaptationFactor    float64 `json:"noise_adaptation_factor"`

	// Веса для многомасштабного анализа
	MultiScaleWeights []float64 `json:"multi_scale_weights"`
}

// TelegramConfig содержит настройки Telegram
type TelegramConfig struct {
	MaxRetries           int  `json:"max_retries"`
	TimeoutSeconds       int  `json:"timeout_seconds"`
	EnableInlineKeyboard bool `json:"enable_inline_keyboard"`
}

// XYPoint представляет точку в 2D пространстве
type XYPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// PolygonData представляет один полигон парковочного места
type PolygonData struct {
	ID     string    `json:"id"`
	Name   string    `json:"name"`
	Points []XYPoint `json:"points"`
}

// PolygonConfig содержит настройки полигонов
type PolygonConfig struct {
	// Параметры трансформации координат
	Scale   float64 `json:"scale"`
	OffsetX float64 `json:"offset_x"`
	OffsetY float64 `json:"offset_y"`

	// Список полигонов
	Polygons []PolygonData `json:"polygons"`
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() *Config {
	return &Config{
		Port:    9991,
		Version: "1.0.0",
		Processing: ProcessingConfig{
			DefaultResizeScale:       0.5,
			DefaultDenoiseStrength:   1.0,
			DefaultContrastBoost:     1.0,
			DayThresholdEmpty:        89.0,
			DayThresholdEdges:        192.0,
			NightThresholdEmpty:      89.0,
			NightThresholdEdges:      128.0,
			EnableAdvancedProcessing: true,
			EnableMultiScale:         true,
			EnableAdaptiveThreshold:  true,
			EnableNoiseReduction:     true,
			PolygonScale:             1.0,
			OffsetX:                  0.0,
			OffsetY:                  0.0,
		},
		Adaptive: AdaptiveConfig{
			LowLightThreshold:        80.0,
			HighContrastThreshold:    50.0,
			NoiseThreshold:           0.1,
			LowLightAdaptationFactor: 0.85,
			BrightAdaptationFactor:   1.1,
			NoiseAdaptationFactor:    1.5,
			MultiScaleWeights:        []float64{0.3, 0.5, 0.2},
		},
		Telegram: TelegramConfig{
			MaxRetries:           3,
			TimeoutSeconds:       30,
			EnableInlineKeyboard: true,
		},
		Polygons: PolygonConfig{
			Scale:   1.0,
			OffsetX: 0.0,
			OffsetY: 0.0,
			Polygons: []PolygonData{
				{
					ID:   "parking_1",
					Name: "Место 1",
					Points: []XYPoint{
						{X: 791, Y: 538},
						{X: 833, Y: 455},
						{X: 873, Y: 472},
						{X: 832, Y: 554},
					},
				},
				{
					ID:   "parking_2",
					Name: "Место 2",
					Points: []XYPoint{
						{X: 835, Y: 554},
						{X: 879, Y: 471},
						{X: 924, Y: 494},
						{X: 880, Y: 577},
					},
				},
				{
					ID:   "parking_3",
					Name: "Место 3",
					Points: []XYPoint{
						{X: 888, Y: 574},
						{X: 930, Y: 488},
						{X: 974, Y: 507},
						{X: 929, Y: 593},
					},
				},
				{
					ID:   "parking_4",
					Name: "Место 4",
					Points: []XYPoint{
						{X: 935, Y: 593},
						{X: 978, Y: 508},
						{X: 1019, Y: 525},
						{X: 981, Y: 615},
					},
				},
				{
					ID:   "parking_5",
					Name: "Место 5",
					Points: []XYPoint{
						{X: 1001, Y: 610},
						{X: 1041, Y: 525},
						{X: 1082, Y: 541},
						{X: 1047, Y: 633},
					},
				},
			},
		},
	}
}

// LoadConfig загружает конфигурацию из файла
func LoadConfig(filename string) (*Config, error) {
	config := DefaultConfig()

	// Проверяем, существует ли файл
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Printf("Config file %s not found, using defaults\n", filename)
		return config, nil
	}

	// Читаем файл
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	// Парсим JSON
	err = json.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %v", err)
	}

	return config, nil
}

// SaveConfig сохраняет конфигурацию в файл
func (c *Config) SaveConfig(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// ToProcessingParams конвертирует конфигурацию в параметры обработки
func (c *Config) ToProcessingParams(isDay bool) ProcessingParams {
	params := ProcessingParams{
		IsDay:           isDay,
		ResizeScale:     c.Processing.DefaultResizeScale,
		DenoiseStrength: c.Processing.DefaultDenoiseStrength,
		ContrastBoost:   c.Processing.DefaultContrastBoost,
	}

	if isDay {
		params.ThresholdEmpty = c.Processing.DayThresholdEmpty
		params.ThresholdEdges = c.Processing.DayThresholdEdges
	} else {
		params.ThresholdEmpty = c.Processing.NightThresholdEmpty
		params.ThresholdEdges = c.Processing.NightThresholdEdges
	}

	// Параметры трансформации полигонов
	params.PolygonScale = c.Polygons.Scale
	params.OffsetX = c.Polygons.OffsetX
	params.OffsetY = c.Polygons.OffsetY

	return params
}

// ToImageProcessor конвертирует конфигурацию в процессор изображений
func (c *Config) ToImageProcessor() *ImageProcessor {
	processor := &ImageProcessor{
		AdaptiveThreshold: c.Processing.EnableAdaptiveThreshold,
		NoiseReduction:    c.Processing.EnableNoiseReduction,
		MultiScale:        c.Processing.EnableMultiScale,
	}

	// Создаем пустые статистики
	processor.HistogramStats = &ImageStats{}

	return processor
}

// GetAdaptiveFactors возвращает коэффициенты адаптации для заданных условий
func (c *Config) GetAdaptiveFactors(stats *ImageStats) (emptyFactor, edgeFactor, noiseFactor float64) {
	emptyFactor = 1.0
	edgeFactor = 1.0
	noiseFactor = 1.0

	if stats.IsLowLight {
		emptyFactor = c.Adaptive.LowLightAdaptationFactor
		edgeFactor = c.Adaptive.LowLightAdaptationFactor
	} else if stats.MeanBrightness > 180 {
		emptyFactor = c.Adaptive.BrightAdaptationFactor
		edgeFactor = c.Adaptive.BrightAdaptationFactor
	}

	if stats.NoiseLevel > c.Adaptive.NoiseThreshold {
		noiseFactor = c.Adaptive.NoiseAdaptationFactor
	}

	return
}

// GetMultiScaleWeights возвращает веса для многомасштабного анализа
func (c *Config) GetMultiScaleWeights() []float64 {
	if len(c.Adaptive.MultiScaleWeights) == 0 {
		return []float64{0.3, 0.5, 0.2} // По умолчанию
	}
	return c.Adaptive.MultiScaleWeights
}

// GetPolygons возвращает полигоны из конфигурации в виде []*poly.Poly
func (c *Config) GetPolygons() []*poly.Poly {
	var result []*poly.Poly

	for _, polygonData := range c.Polygons.Polygons {
		polyPoints := make([]poly.XY, len(polygonData.Points))
		for i, point := range polygonData.Points {
			// Применяем трансформацию координат
			x := point.X*c.Polygons.Scale + c.Polygons.OffsetX
			y := point.Y*c.Polygons.Scale + c.Polygons.OffsetY
			polyPoints[i] = poly.XY{X: x, Y: y}
		}

		result = append(result, &poly.Poly{
			XY: polyPoints,
		})
	}

	return result
}

// UpdatePolygon обновляет полигон в конфигурации
func (c *Config) UpdatePolygon(id string, points []XYPoint) error {
	for i, polygon := range c.Polygons.Polygons {
		if polygon.ID == id {
			c.Polygons.Polygons[i].Points = points
			return nil
		}
	}
	return fmt.Errorf("polygon with id %s not found", id)
}

// AddPolygon добавляет новый полигон в конфигурацию
func (c *Config) AddPolygon(id, name string, points []XYPoint) error {
	// Проверяем, что ID уникален
	for _, polygon := range c.Polygons.Polygons {
		if polygon.ID == id {
			return fmt.Errorf("polygon with id %s already exists", id)
		}
	}

	newPolygon := PolygonData{
		ID:     id,
		Name:   name,
		Points: points,
	}

	c.Polygons.Polygons = append(c.Polygons.Polygons, newPolygon)
	return nil
}

// RemovePolygon удаляет полигон из конфигурации
func (c *Config) RemovePolygon(id string) error {
	for i, polygon := range c.Polygons.Polygons {
		if polygon.ID == id {
			c.Polygons.Polygons = append(c.Polygons.Polygons[:i], c.Polygons.Polygons[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("polygon with id %s not found", id)
}

// UpdatePolygonTransform обновляет параметры трансформации полигонов
func (c *Config) UpdatePolygonTransform(scale, offsetX, offsetY float64) {
	c.Polygons.Scale = scale
	c.Polygons.OffsetX = offsetX
	c.Polygons.OffsetY = offsetY
}

// Глобальная переменная для конфигурации
var appConfig *Config

// InitConfig инициализирует конфигурацию
func InitConfig() error {
	var err error
	appConfig, err = LoadConfig("advanced_config.json")
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	// Сохраняем конфигурацию по умолчанию для примера
	if _, err := os.Stat("advanced_config.json"); os.IsNotExist(err) {
		err = appConfig.SaveConfig("advanced_config.json")
		if err != nil {
			fmt.Printf("Warning: failed to save default config: %v\n", err)
		} else {
			fmt.Println("Created default config file: advanced_config.json")
		}
	}

	return nil
}
