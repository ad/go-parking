package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ad/go-parking/poly"
	"github.com/ernyoke/imger/edgedetection"
	"github.com/ernyoke/imger/effects"
	"github.com/ernyoke/imger/grayscale"
	"github.com/ernyoke/imger/resize"

	"github.com/fogleman/gg"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/goregular"
)

// Структуры для API работы с полигонами
type PolygonPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type PolygonAPI struct {
	ID     int64          `json:"id"`
	Points []PolygonPoint `json:"points"`
}

type PolygonsResponse struct {
	Success  bool         `json:"success"`
	Polygons []PolygonAPI `json:"polygons"`
	Error    string       `json:"error,omitempty"`
}

type PolygonsRequest struct {
	Polygons []PolygonAPI `json:"polygons"`
}

type SaveResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// Структуры для данных шаблонов
type FormData struct {
	Target       string
	Token        string
	Day          bool
	PolygonScale float64
	OffsetX      float64
	OffsetY      float64
}

type PreviewData struct {
	PolygonCount int
	PolygonScale float64
	OffsetX      float64
	OffsetY      float64
	PolygonList  template.HTML
	Result       template.HTML
}

type ConfigData struct {
	Port                    int
	Version                 string
	ProcessingStatus        string
	ProcessingText          string
	MultiScaleStatus        string
	MultiScaleText          string
	NoiseReductionStatus    string
	NoiseReductionText      string
	AdaptiveThresholdStatus string
	AdaptiveThresholdText   string
	DayThreshold            float64
	NightThreshold          float64
	DenoiseStrength         float64
	MultiScaleCount         int
}

// Глобальные переменные для шаблонов
var (
	formTemplate    *template.Template
	previewTemplate *template.Template
	configTemplate  *template.Template
)

// Функция для загрузки шаблонов
func loadTemplates() error {
	var err error

	formTemplate, err = template.ParseFiles("templates/form.html")
	if err != nil {
		return fmt.Errorf("error loading form template: %w", err)
	}

	previewTemplate, err = template.ParseFiles("templates/preview.html")
	if err != nil {
		return fmt.Errorf("error loading preview template: %w", err)
	}

	configTemplate, err = template.ParseFiles("templates/config.html")
	if err != nil {
		return fmt.Errorf("error loading config template: %w", err)
	}

	return nil
}

var version string

// Функция для трансформации полигонов с учетом масштаба и смещения
func transformPolygons(polygons []*poly.Poly, scale, offsetX, offsetY float64) []*poly.Poly {
	transformed := make([]*poly.Poly, len(polygons))
	for i, p := range polygons {
		newPoly := &poly.Poly{
			XY: make([]poly.XY, len(p.XY)),
		}
		for j, point := range p.XY {
			newPoly.XY[j] = poly.XY{
				X: point.X*scale + offsetX,
				Y: point.Y*scale + offsetY,
			}
		}
		transformed[i] = newPoly
	}
	return transformed
}

func main() {
	// Инициализируем конфигурацию
	err := InitConfig()
	if err != nil {
		fmt.Printf("Failed to initialize config: %v\n", err)
		return
	}

	// Загружаем шаблоны
	err = loadTemplates()
	if err != nil {
		fmt.Printf("Failed to load templates: %v\n", err)
		return
	}

	mux := http.NewServeMux()

	// return form for uploading image
	mux.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("get form...")
		polygons := appConfig.GetPolygons()
		if len(polygons) > 0 {
			polygon := polygons[0]
			min, max := polygon.MinMax()
			fmt.Println("minMax:", min, max)
		}

		// Извлекаем параметры из URL или устанавливаем значения по умолчанию
		data := FormData{
			Target:       r.FormValue("target"),
			Token:        r.FormValue("token"),
			Day:          r.FormValue("day") != "",
			PolygonScale: 1.0,
			OffsetX:      0,
			OffsetY:      0,
		}

		// Парсим значения масштаба и смещения если они есть
		if scale := r.FormValue("polygon_scale"); scale != "" {
			if s, err := strconv.ParseFloat(scale, 64); err == nil {
				data.PolygonScale = s
			}
		}
		if offsetX := r.FormValue("offset_x"); offsetX != "" {
			if x, err := strconv.ParseFloat(offsetX, 64); err == nil {
				data.OffsetX = x
			}
		}
		if offsetY := r.FormValue("offset_y"); offsetY != "" {
			if y, err := strconv.ParseFloat(offsetY, 64); err == nil {
				data.OffsetY = y
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		formTemplate.Execute(w, data)
	})

	mux.HandleFunc("/process", processImage)

	// Добавляем endpoint для предварительного просмотра полигонов
	mux.HandleFunc("/preview", previewPolygons)
	// Добавляем endpoint для получения конфигурации
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// Показываем HTML страницу конфигурации
			showConfigPage(w, r)
			return
		}

		// Для POST запросов возвращаем JSON
		w.Header().Set("Content-Type", "application/json")
		data, err := json.MarshalIndent(appConfig, "", "  ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(data)
	})

	// API endpoints для работы с полигонами
	mux.HandleFunc("/api/polygons", handlePolygonsAPI)
	mux.HandleFunc("/api/polygons/save", handleSavePolygons)

	// Новый обработчик для кнопки "Просмотр работы алгоритма"
	mux.HandleFunc("/view_parking_algorithm", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("=== Incoming request to /view_parking_algorithm ===\n")
		fmt.Printf("Method: %s, URL: %s\n", r.Method, r.URL.Path)
		viewParkingAlgorithm(w, r)
	})

	port := appConfig.Port
	if port == 0 {
		port = 9991
	}

	fmt.Printf("Server v%s is running on localhost:%d\n", version, port)
	fmt.Printf("Advanced processing enabled: %t\n", appConfig.Processing.EnableAdvancedProcessing)
	fmt.Printf("Multi-scale analysis enabled: %t\n", appConfig.Processing.EnableMultiScale)

	http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), mux)
}

func processImage(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	fmt.Println("Processing image...")

	isDay := true
	if r.FormValue("day") == "1" {
		isDay = true
	}

	// Получаем параметры трансформации полигонов
	polygonScale := 1.0
	offsetX := 0.0
	offsetY := 0.0

	if scale := r.FormValue("polygon_scale"); scale != "" {
		if s, err := strconv.ParseFloat(scale, 64); err == nil {
			polygonScale = s
		}
	}
	if offX := r.FormValue("offset_x"); offX != "" {
		if x, err := strconv.ParseFloat(offX, 64); err == nil {
			offsetX = x
		}
	}
	if offY := r.FormValue("offset_y"); offY != "" {
		if y, err := strconv.ParseFloat(offY, 64); err == nil {
			offsetY = y
		}
	}

	// get file from form
	file, _, err := r.FormFile("file")
	if err != nil {
		fmt.Printf("r.FormFile error: %s", err.Error())
		return
	}
	defer file.Close()

	// convert file to image
	img, err := decodeImage(file)
	if err != nil {
		fmt.Printf("image.Decode error: %s", err.Error())
		return
	}

	// Получаем полигоны из конфигурации
	polygons := appConfig.GetPolygons()

	// Используем конфигурацию для создания параметров
	params := appConfig.ToProcessingParams(isDay)
	params.PolygonScale = polygonScale
	params.OffsetX = offsetX
	params.OffsetY = offsetY

	// Используем общую функцию для обработки изображения
	imgRGBA, err := analyzeParkingSpaces(img, polygons, params, isDay)
	if err != nil {
		fmt.Printf("could not analyze parking spaces: %s", err.Error())
		return
	}

	processingTime := time.Since(start)
	fmt.Printf("Advanced processing took %s\n", processingTime)

	chatID, err := strconv.ParseInt(r.FormValue("target"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isUpdate := false
	if r.FormValue("update") == "1" {
		isUpdate = true
	}

	token := r.FormValue("token")

	if isUpdate {
		messageID := r.FormValue("message_id")
		threadID := r.FormValue("thread_id")
		sendImageUpdateToTelegram(imgRGBA, chatID, messageID, threadID, token)
		return
	}

	sendImageTotelegram(imgRGBA, chatID, token)
}

// Вынесенная функция для обработки изображения
func analyzeParkingSpaces(img image.Image, polygons []*poly.Poly, params ProcessingParams, isDay bool) (*image.RGBA, error) {
	b := img.Bounds()
	imgRGBA := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(imgRGBA, imgRGBA.Bounds(), img, b.Min, draw.Src)

	processor := appConfig.ToImageProcessor()
	imgGG := gg.NewContextForRGBA(imgRGBA)
	imgGG.SetLineWidth(2)

	// Полная обработка изображения
	grayscaleImg, stats, err := processImageCore(imgRGBA, processor, params, isDay)
	if err != nil {
		return nil, err
	}

	// Edge detection с адаптивными порогами
	adaptedEdgeThreshold := params.ThresholdEdges
	if stats.IsLowLight {
		adaptedEdgeThreshold *= 0.8
	}

	imgEdges, errCanny := edgedetection.CannyGray(grayscaleImg, 1, adaptedEdgeThreshold, 1)
	if errCanny != nil {
		return nil, fmt.Errorf("could not detect edges: %w", errCanny)
	}

	// Invert image
	imgEdges = effects.InvertGray(imgEdges)

	// save edges image for debugging
	if appConfig.Processing.EnableDebugVisualization {
		edgesFile, err := os.Create("debug_edges.png")
		if err != nil {
			return nil, fmt.Errorf("could not create debug edges image: %w", err)
		}
		defer edgesFile.Close()
		err = jpeg.Encode(edgesFile, imgEdges, &jpeg.Options{Quality: 90})
		if err != nil {
			return nil, fmt.Errorf("could not save debug edges image: %w", err)
		}
		fmt.Println("Debug edges image saved as debug_edges.png")
	}

	// Трансформируем полигоны для анализа на уменьшенном изображении
	// Если изображение уменьшается в ResizeScale раз, то полигоны тоже должны быть уменьшены
	analysisPolygons := transformPolygons(polygons, params.PolygonScale*params.ResizeScale, params.OffsetX*params.ResizeScale, params.OffsetY*params.ResizeScale)

	// Трансформируем полигоны для отрисовки на оригинальном изображении
	transformedPolygons := transformPolygons(polygons, params.PolygonScale, params.OffsetX, params.OffsetY)

	// Адаптируем порог занятости на основе статистик
	adaptedEmptyThreshold := params.ThresholdEmpty
	if stats.IsLowLight {
		adaptedEmptyThreshold *= 0.9
	}

	min, max := poly.MinMaxMany(transformedPolygons)

	// Анализируем полигоны
	fmt.Printf("Начинаем анализ %d полигонов...\n", len(transformedPolygons))
	fmt.Printf("Границы изображения: min=(%.1f,%.1f), max=(%.1f,%.1f)\n", min.X, min.Y, max.X, max.Y)
	fmt.Printf("Размер изображения после edge detection: %dx%d\n", imgEdges.Bounds().Dx(), imgEdges.Bounds().Dy())
	fmt.Printf("Используется многомасштабный анализ: %t\n", processor.MultiScale)

	for i, polygon := range analysisPolygons {
		// Сбрасываем счетчики
		polygon.Zero = 0
		polygon.NonZero = 0

		fmt.Printf("\nАнализируем полигон %d:\n", i+1)

		// Проверяем, пересекается ли полигон с уменьшенным изображением
		bounds := imgEdges.Bounds()
		min, max := polygon.MinMax()

		fmt.Printf("  Границы полигона: (%.1f,%.1f) - (%.1f,%.1f)\n", min.X, min.Y, max.X, max.Y)
		fmt.Printf("  Границы изображения: (%d,%d) - (%d,%d)\n", bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)

		// Теперь полигон уже правильно масштабирован для уменьшенного изображения
		// Не нужно дополнительное масштабирование

		// Проверяем пересечение с уменьшенным изображением
		if max.X < float64(bounds.Min.X) || min.X > float64(bounds.Max.X) ||
			max.Y < float64(bounds.Min.Y) || min.Y > float64(bounds.Max.Y) {
			fmt.Printf("  Полигон полностью за пределами уменьшенного изображения, помечаем как занятый\n")
			// Полигон вне уменьшенного изображения считаем занятым
			polygon.Zero = 0
			polygon.NonZero = 1000
			continue
		}

		// Многомасштабный анализ для точного определения
		if processor.MultiScale {
			fmt.Printf("  Используем многомасштабный анализ...\n")
			results := processor.MultiScaleAnalysis(grayscaleImg, polygon, params)
			weightedResult := processor.GetWeightedResult(results)

			fmt.Printf("  Результат многомасштабного анализа: %.2f%%\n", weightedResult)

			if weightedResult >= 0 {
				// Конвертируем процент в счетчики
				totalPixels := 1000
				freePixels := int(float64(totalPixels) * weightedResult / 100)
				occupiedPixels := totalPixels - freePixels

				polygon.Zero = freePixels
				polygon.NonZero = occupiedPixels
				fmt.Printf("  Многомасштабный анализ: Zero=%d, NonZero=%d\n", polygon.Zero, polygon.NonZero)
				continue
			} else {
				fmt.Printf("  Многомасштабный анализ не смог обработать полигон\n")
			}
		}

		// Если многомасштабный анализ не дал результатов, используем упрощенный
		fmt.Printf("  Используем упрощенный анализ...\n")

		// Упрощенный анализ с прямым обходом пикселей
		lightPixels := 0
		darkPixels := 0
		totalSamples := 0

		// Используем адаптивный порог вместо hardcoded 255
		threshold := uint8(10) // Средний уровень серого

		// Проходим по всем пикселям изображения
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				// Полигон уже правильно масштабирован для уменьшенного изображения
				// Не нужно дополнительное масштабирование координат
				point := poly.XY{X: float64(x), Y: float64(y)}
				if point.In(*polygon) {
					totalSamples++
					pixel := imgEdges.GrayAt(x, y)

					if pixel.Y >= threshold {
						lightPixels++
						// draw light pixel on result image
						imgGG.Image().(*image.RGBA).SetRGBA(x, y, color.RGBA{0, 255, 0, 255}) // Зеленый для светлых пикселей
					} else {
						darkPixels++
						// draw dark pixel on result image
						imgGG.Image().(*image.RGBA).SetRGBA(x, y, color.RGBA{255, 0, 0, 255}) // Красный для темных пикселей
					}
				}
			}
		}

		fmt.Printf("  Упрощенный анализ: samples=%d, light=%d, dark=%d\n", totalSamples, lightPixels, darkPixels)

		if totalSamples > 0 {
			polygon.Zero = lightPixels
			polygon.NonZero = darkPixels
		} else {
			fmt.Printf("  Упрощенный анализ не нашел пикселей, полигон вне изображения\n")
			// Полигон не пересекается с изображением - считаем занятым
			polygon.Zero = 0
			polygon.NonZero = 1000
		}
	}

	// Копируем результаты анализа из analysisPolygons в transformedPolygons для отрисовки
	for i := range transformedPolygons {
		transformedPolygons[i].Zero = analysisPolygons[i].Zero
		transformedPolygons[i].NonZero = analysisPolygons[i].NonZero
	}

	// Настраиваем шрифт для отрисовки
	font, _ := truetype.Parse(goregular.TTF)
	face := truetype.NewFace(font, &truetype.Options{Size: 18})
	imgGG.SetFontFace(face)

	// Отрисовываем результаты
	fmt.Printf("Отрисовываем %d полигонов...\n", len(transformedPolygons))

	for i, polygon := range transformedPolygons {
		center := polygon.Center()
		fmt.Printf("Полигон %d: center=(%.1f, %.1f), Zero=%d, NonZero=%d\n",
			i+1, center.X, center.Y, polygon.Zero, polygon.NonZero)

		// Всегда отрисовываем полигон, независимо от анализа
		var col color.RGBA
		var percentage float64

		if polygon.Zero > 0 {
			// Правильная формула: процент свободных мест = Zero / (Zero + NonZero) * 100
			totalPixels := polygon.Zero + polygon.NonZero
			percentage = float64(polygon.Zero) / float64(totalPixels) * 100
			fmt.Printf("Полигон %d: %.1f%% свободно (Zero=%d, NonZero=%d, Total=%d, порог: %.1f)\n",
				i+1, percentage, polygon.Zero, polygon.NonZero, totalPixels, adaptedEmptyThreshold)

			if percentage > adaptedEmptyThreshold {
				col = color.RGBA{0, 255, 0, 255} // Зеленый для свободных
			} else {
				col = color.RGBA{255, 0, 0, 255} // Красный для занятых
			}
		} else {
			// Если анализ не дал результатов, рисуем синим
			col = color.RGBA{0, 0, 255, 255}
			percentage = 0
			fmt.Printf("Полигон %d: нет данных анализа, рисуем синим\n", i+1)
		}

		// Рисуем полигон
		DrawPolygon(imgGG, polygon, col, 5)
		fmt.Printf("Полигон %d нарисован цветом RGB(%d,%d,%d)\n", i+1, col.R, col.G, col.B)

		// // Отрисовываем точки анализа если включена debug визуализация
		// // Показываем точки только для первых 3 полигонов, чтобы не загромождать изображение
		// if appConfig.Processing.EnableDebugVisualization && polygon.Zero > 0 && polygon.NonZero > 0 {
		// 	// Используем контрастные цвета с хорошей видимостью
		// 	lightPointColor := color.RGBA{0, 255, 0, 255} // Ярко-зеленый для светлых пикселей (свободно)
		// 	darkPointColor := color.RGBA{255, 0, 0, 255}  // Ярко-красный для темных пикселей (занято)
		// 	threshold := uint8(10)                        // Тот же порог, что используется в анализе
		// 	step := 5                                     // Меньший шаг для большего количества точек

		// 	fmt.Printf("  Отрисовываем debug точки для полигона %d (step=%d, threshold=%d)\n", i+1, step, threshold)
		// 	// Отрисовываем точки с масштабированием
		// 	DrawScaledAnalysisPoints(imgGG, imgEdges, analysisPolygons[i], lightPointColor, darkPointColor, threshold, step, params.ResizeScale)
		// }

		// Добавляем текст с процентами или номером
		// Для полигонов за пределами изображения (Zero=0, NonZero=1000) показываем номер
		if polygon.Zero == 0 && polygon.NonZero == 1000 {
			// Полигон за пределами изображения - показываем номер
			DrawStrokeText(imgGG, fmt.Sprintf("%d", i+1), center.X, center.Y,
				color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255}, 3)
		} else if polygon.Zero > 0 {
			// Полигон с данными анализа - показываем процент
			DrawStrokeText(imgGG, fmt.Sprintf("%.1f", percentage), center.X, center.Y,
				color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}, 3)
		} else {
			// Полигон без данных (ошибка анализа) - показываем номер
			DrawStrokeText(imgGG, fmt.Sprintf("%d", i+1), center.X, center.Y,
				color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255}, 3)
		}
	}

	return imgGG.Image().(*image.RGBA), nil
}

func sendImageTotelegram(img image.Image, chatID int64, botToken string) {
	// file, err := os.Create("output.jpg")
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// defer file.Close()

	// err = jpeg.Encode(file, img, nil)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto?chat_id=%d", botToken, chatID)

	resp, err := upload(url, img)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	// fmt.Println(string(body))
}

func sendImageUpdateToTelegram(img image.Image, chatID int64, messageID, threadID, botToken string) {
	// file, err := os.Create("output.jpg")
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// defer file.Close()

	// err = jpeg.Encode(file, img, nil)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	replyKeyboard := "reply_markup=%7B%22inline_keyboard%22%3A%20%5B%5B%7B%22text%22%3A%20%22Update%20🤓%22%2C%22callback_data%22%3A%20%22%2Fcamera_update%22%7D%5D%5D%7D&media=%7B%22type%22%3A%20%22photo%22%2C%20%22media%22%3A%22attach%3A%2F%2Fphoto%22%7D"

	url := fmt.Sprintf(
		"https://api.telegram.org/bot%s/editMessageMedia?chat_id=%d&message_id=%smessage_thread_id=%s&disable_notification=true&%s",
		botToken,
		chatID,
		messageID,
		threadID,
		replyKeyboard,
	)

	resp, err := upload(url, img)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	// fmt.Println(string(body))
}

func upload(url string, img image.Image) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "multipart/form-data")

	var b bytes.Buffer
	multipartWriter := multipart.NewWriter(&b)
	fileWriter, err := multipartWriter.CreateFormFile("photo", "photo.jpg")
	if err != nil {
		return nil, err
	}
	err = jpeg.Encode(fileWriter, img, nil)
	if err != nil {
		return nil, err
	}
	multipartWriter.Close()

	contentType := fmt.Sprintf("multipart/form-data; boundary=%s", multipartWriter.Boundary())
	req.Header.Set("Content-Type", contentType)
	// req.Header.Set("Content-Length", fmt.Sprintf("%d", body.Len()))

	req.Body = io.NopCloser(&b)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	fmt.Println(resp.Status)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad status: %s, %s", resp.Status, resp.Body)
	}

	return resp, nil
}

func previewPolygons(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// Показываем форму для загрузки изображения
		showPreviewForm(w, r, "")
		return
	}

	if r.Method == "POST" {
		// Обрабатываем загруженное изображение
		processPreview(w, r)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func showPreviewForm(w http.ResponseWriter, r *http.Request, result string) {
	// Получаем параметры масштаба и смещения
	polygonScale := 1.0
	offsetX := 0.0
	offsetY := 0.0

	if scale := r.FormValue("polygon_scale"); scale != "" {
		if s, err := strconv.ParseFloat(scale, 64); err == nil {
			polygonScale = s
		}
	}
	if offX := r.FormValue("offset_x"); offX != "" {
		if x, err := strconv.ParseFloat(offX, 64); err == nil {
			offsetX = x
		}
	}
	if offY := r.FormValue("offset_y"); offY != "" {
		if y, err := strconv.ParseFloat(offY, 64); err == nil {
			offsetY = y
		}
	}

	// Получаем полигоны из конфигурации
	polygons := appConfig.GetPolygons()

	// Подготавливаем данные для шаблона
	data := PreviewData{
		PolygonCount: len(polygons),
		PolygonScale: polygonScale,
		OffsetX:      offsetX,
		OffsetY:      offsetY,
		Result:       template.HTML(result),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	previewTemplate.Execute(w, data)
}

func processPreview(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	fmt.Println("Processing preview image...")

	// Получаем параметры трансформации полигонов
	polygonScale := 1.0
	offsetX := 0.0
	offsetY := 0.0

	if scale := r.FormValue("polygon_scale"); scale != "" {
		if s, err := strconv.ParseFloat(scale, 64); err == nil {
			polygonScale = s
		}
	}
	if offX := r.FormValue("offset_x"); offX != "" {
		if x, err := strconv.ParseFloat(offX, 64); err == nil {
			offsetX = x
		}
	}
	if offY := r.FormValue("offset_y"); offY != "" {
		if y, err := strconv.ParseFloat(offY, 64); err == nil {
			offsetY = y
		}
	}

	// Получаем файл из формы
	file, filename, err := r.FormFile("file")
	if err != nil {
		showPreviewForm(w, r, fmt.Sprintf(`<div style="color: red;">Ошибка загрузки файла: %s</div>`, err.Error()))
		return
	}
	defer file.Close()

	// Конвертируем файл в изображение
	img, err := decodeImage(file)
	if err != nil {
		showPreviewForm(w, r, fmt.Sprintf(`<div style="color: red;">Ошибка декодирования изображения: %s</div>`, err.Error()))
		return
	}

	fmt.Printf("Loaded image: %s, size: %dx%d\n",
		filename.Filename, img.Bounds().Dx(), img.Bounds().Dy())
	fmt.Printf("Using polygon scale: %.2f, offset: (%.0f, %.0f)\n", polygonScale, offsetX, offsetY)

	// Создаем RGBA изображение для рисования
	b := img.Bounds()
	imgRGBA := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(imgRGBA, imgRGBA.Bounds(), img, b.Min, draw.Src)

	// Получаем полигоны из конфигурации и трансформируем их для отображения
	polygons := appConfig.GetPolygons()
	transformedPolygons := transformPolygons(polygons, polygonScale, offsetX, offsetY)

	// Создаем графический контекст
	imgGG := gg.NewContextForRGBA(imgRGBA)

	// Настраиваем шрифт
	font, err := truetype.Parse(goregular.TTF)
	if err == nil {
		face := truetype.NewFace(font, &truetype.Options{Size: 16})
		imgGG.SetFontFace(face)
	}

	for i, polygon := range transformedPolygons {
		polygonColor := color.RGBA{0, 0, 0, 255}

		// Рисуем полигон
		DrawPolygon(imgGG, polygon, polygonColor, 3)

		// Добавляем номер парковочного места
		center := polygon.Center()
		textColor := color.RGBA{0, 0, 0, 255}
		strokeColor := color.RGBA{255, 255, 255, 255}
		DrawStrokeText(imgGG, fmt.Sprintf("%d", i+1), center.X, center.Y,
			textColor, strokeColor, 3)
	}

	// Добавляем информацию о изображении
	imgGG.SetColor(color.RGBA{0, 0, 0, 200})
	imgGG.DrawRectangle(10, 10, 400, 120)
	imgGG.Fill()

	imgGG.SetColor(color.RGBA{255, 255, 255, 255})
	imgGG.DrawString(fmt.Sprintf("Полигонов: %d", len(polygons)), 20, 30)
	imgGG.DrawString(fmt.Sprintf("Размер: %dx%d", b.Dx(), b.Dy()), 20, 50)
	imgGG.DrawString(fmt.Sprintf("Формат: %s", "JPEG"), 20, 70)
	imgGG.DrawString(fmt.Sprintf("Масштаб: %.2f, Смещение: (%.0f,%.0f)", polygonScale, offsetX, offsetY), 20, 90)
	imgGG.DrawString(fmt.Sprintf("Время: %s", time.Since(start)), 20, 110)

	processingTime := time.Since(start)
	fmt.Printf("Preview processing took %s\n", processingTime)

	// Конвертируем результат в base64 для отображения в HTML
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, imgGG.Image(), &jpeg.Options{Quality: 85})
	if err != nil {
		showPreviewForm(w, r, fmt.Sprintf(`<div style="color: red;">Ошибка кодирования результата: %s</div>`, err.Error()))
		return
	}

	// Кодируем в base64
	encoded := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	// Показываем результат
	result := fmt.Sprintf(`
		<h3>Результат обработки:</h3>
		<p><strong>Время обработки:</strong> %s</p>
		<p><strong>Найдено полигонов:</strong> %d</p>
		<img src="%s" alt="Preview with polygons" />
	`, processingTime, len(polygons), encoded)

	showPreviewForm(w, r, result)
}

func showConfigPage(w http.ResponseWriter, r *http.Request) {
	versionStr := version
	if versionStr == "" {
		versionStr = "development"
	}

	enabledStatus := func(enabled bool) string {
		if enabled {
			return "enabled"
		}
		return "disabled"
	}

	enabledText := func(enabled bool) string {
		if enabled {
			return "Включено"
		}
		return "Выключено"
	}

	data := ConfigData{
		Port:                    appConfig.Port,
		Version:                 versionStr,
		ProcessingStatus:        enabledStatus(appConfig.Processing.EnableAdvancedProcessing),
		ProcessingText:          enabledText(appConfig.Processing.EnableAdvancedProcessing),
		MultiScaleStatus:        enabledStatus(appConfig.Processing.EnableMultiScale),
		MultiScaleText:          enabledText(appConfig.Processing.EnableMultiScale),
		NoiseReductionStatus:    enabledStatus(appConfig.Processing.EnableNoiseReduction),
		NoiseReductionText:      enabledText(appConfig.Processing.EnableNoiseReduction),
		AdaptiveThresholdStatus: enabledStatus(appConfig.Processing.EnableAdaptiveThreshold),
		AdaptiveThresholdText:   enabledText(appConfig.Processing.EnableAdaptiveThreshold),
		DayThreshold:            appConfig.Processing.DayThresholdEmpty,
		NightThreshold:          appConfig.Processing.NightThresholdEmpty,
		DenoiseStrength:         appConfig.Processing.DefaultDenoiseStrength,
		MultiScaleCount:         len(appConfig.Adaptive.MultiScaleWeights),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	configTemplate.Execute(w, data)
}

// Обработчик API для работы с полигонами
func handlePolygonsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "GET" {
		// Получаем полигоны из конфигурации
		polygons := appConfig.GetPolygons()

		// Возвращаем текущие полигоны
		apiPolygons := make([]PolygonAPI, len(polygons))
		for i, polygon := range polygons {
			apiPolygons[i] = PolygonAPI{
				ID:     int64(i),
				Points: make([]PolygonPoint, len(polygon.XY)),
			}
			for j, point := range polygon.XY {
				apiPolygons[i].Points[j] = PolygonPoint{
					X: point.X,
					Y: point.Y,
				}
			}
		}

		response := PolygonsResponse{
			Success:  true,
			Polygons: apiPolygons,
		}

		json.NewEncoder(w).Encode(response)
		return
	}

	// Для POST запросов перенаправляем на правильный endpoint
	if r.Method == "POST" {
		http.Error(w, "Use /api/polygons/save for saving polygons", http.StatusBadRequest)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// Обработчик для сохранения полигонов
func handleSavePolygons(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var request PolygonsRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		fmt.Printf("Error decoding request: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Printf("Received request to save %d polygons to configuration\n", len(request.Polygons))

	// Очищаем существующие полигоны
	appConfig.Polygons.Polygons = []PolygonData{}

	// Добавляем новые полигоны из запроса
	for i, apiPolygon := range request.Polygons {
		points := make([]XYPoint, len(apiPolygon.Points))
		for j, point := range apiPolygon.Points {
			points[j] = XYPoint(point)
		}

		polygonData := PolygonData{
			ID:     fmt.Sprintf("parking_%d", i+1),
			Name:   fmt.Sprintf("Место %d", i+1),
			Points: points,
		}

		appConfig.Polygons.Polygons = append(appConfig.Polygons.Polygons, polygonData)

		if i < 3 {
			fmt.Printf("Added polygon %d with %d points: %v\n", i+1, len(points), points)
		}
	}

	// Сохраняем конфигурацию
	fmt.Printf("Saving configuration to advanced_config.json...\n")
	err = appConfig.SaveConfig("advanced_config.json")
	if err != nil {
		fmt.Printf("Failed to save configuration: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Printf("Successfully saved %d polygons to configuration\n", len(request.Polygons))

	response := map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Saved %d polygons to configuration", len(request.Polygons)),
		"count":   len(request.Polygons),
	}

	json.NewEncoder(w).Encode(response)
}

// Функция для сохранения полигонов в файл
func savePolygonsToFile(polygons []*poly.Poly) error {
	// Создаем резервную копию
	if err := backupPolygonsFile(); err != nil {
		fmt.Printf("Warning: Could not create backup: %v\n", err)
	}

	// Генерируем новый код для polygons.go
	code := generatePolygonsCode(polygons)

	// Записываем в файл
	err := os.WriteFile("polygons.go", []byte(code), 0644)
	if err != nil {
		return fmt.Errorf("failed to write polygons file: %w", err)
	}

	fmt.Printf("Saved %d polygons to polygons.go\n", len(polygons))
	return nil
}

// Создание резервной копии файла полигонов
func backupPolygonsFile() error {
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("polygons_backup_%s.go", timestamp)

	input, err := os.ReadFile("polygons.go")
	if err != nil {
		return err
	}

	return os.WriteFile(backupName, input, 0644)
}

// Генерация кода для файла polygons.go
func generatePolygonsCode(polygons []*poly.Poly) string {
	var code strings.Builder

	code.WriteString("package main\n\n")
	code.WriteString("import (\n")
	code.WriteString("\t\"github.com/ad/go-parking/poly\"\n")
	code.WriteString(")\n\n")
	code.WriteString("var polygons = []*poly.Poly{\n")

	for i, polygon := range polygons {
		code.WriteString("\t{\n")
		code.WriteString("\t\tXY: []poly.XY{\n")

		for _, point := range polygon.XY {
			code.WriteString(fmt.Sprintf("\t\t\t{X: %.1f, Y: %.1f},\n", point.X, point.Y))
		}

		code.WriteString("\t\t},\n")
		code.WriteString("\t}")

		if i < len(polygons)-1 {
			code.WriteString(",")
		}
		code.WriteString("\n")
	}

	code.WriteString("}\n")
	return code.String()
}

func processImageCore(imgRGBA *image.RGBA, processor *ImageProcessor, params ProcessingParams, isDay bool) (*image.Gray, *ImageStats, error) {
	var grayscaleImg *image.Gray
	var stats *ImageStats

	// Используем улучшенную обработку если включена
	if appConfig.Processing.EnableAdvancedProcessing {
		processedImg, imgStats, err := processor.ProcessImageAdvanced(imgRGBA, params)
		if err != nil {
			return nil, nil, fmt.Errorf("could not process image: %w", err)
		}
		grayscaleImg = processedImg.(*image.Gray)
		stats = imgStats

		// Логируем статистики для отладки
		fmt.Printf("Image stats: brightness=%.2f, contrast=%.2f, noise=%.2f, lowLight=%t\n",
			stats.MeanBrightness, stats.Contrast, stats.NoiseLevel, stats.IsLowLight)
	} else {
		// Стандартная обработка как резерв
		grayscaleImg = grayscale.Grayscale(imgRGBA)
		sharpened, err := effects.SharpenGray(grayscaleImg)
		if err != nil {
			return nil, nil, fmt.Errorf("could not sharpen image: %w", err)
		}
		grayscaleImg = sharpened

		// Создаем базовые статистики
		stats = &ImageStats{
			MeanBrightness: 128,
			Contrast:       30,
			NoiseLevel:     0.05,
			IsLowLight:     !isDay,
			IsHighContrast: false,
		}
	}

	// Дополнительное масштабирование если нужно
	if params.ResizeScale != 1.0 {
		var err error
		grayscaleImg, err = resize.ResizeGray(grayscaleImg, params.ResizeScale, params.ResizeScale, resize.InterNearest)
		if err != nil {
			return nil, nil, fmt.Errorf("could not resize image: %w", err)
		}
	}

	return grayscaleImg, stats, nil
}

// Функция для декодирования изображения из файла
func decodeImage(file multipart.File) (image.Image, error) {
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("image.Decode error: %w", err)
	}
	return img, nil
}

// Функция для отображения формы с результатом
func showFormWithResult(w http.ResponseWriter, r *http.Request, result string) {
	// Получаем параметры масштаба и смещения
	polygonScale := 1.0
	offsetX := 0.0
	offsetY := 0.0

	if scale := r.FormValue("polygon_scale"); scale != "" {
		if s, err := strconv.ParseFloat(scale, 64); err == nil {
			polygonScale = s
		}
	}
	if offX := r.FormValue("offset_x"); offX != "" {
		if x, err := strconv.ParseFloat(offX, 64); err == nil {
			offsetX = x
		}
	}
	if offY := r.FormValue("offset_y"); offY != "" {
		if y, err := strconv.ParseFloat(offY, 64); err == nil {
			offsetY = y
		}
	}

	// Подготавливаем данные для шаблона
	data := FormData{
		Target:       r.FormValue("target"),
		Token:        r.FormValue("token"),
		Day:          r.FormValue("day") == "1",
		PolygonScale: polygonScale,
		OffsetX:      offsetX,
		OffsetY:      offsetY,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Создаем HTML с результатом
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>go-parking - Upload</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 800px; margin: 0 auto; background: white; padding: 30px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; text-align: center; }
        form { margin-top: 20px; }
        input[type="text"], input[type="file"] { width: 100%%; padding: 10px; margin: 10px 0; border: 1px solid #ddd; border-radius: 5px; }
        input[type="submit"] { background: #007bff; color: white; padding: 12px 20px; border: none; border-radius: 5px; cursor: pointer; margin-right: 10px; }
        input[type="submit"]:hover { background: #0056b3; }
        input[type="submit"].algorithm { background: #28a745; }
        input[type="submit"].algorithm:hover { background: #218838; }
        .nav { text-align: center; margin-bottom: 20px; }
        .nav a { margin: 0 10px; padding: 8px 16px; background: #6c757d; color: white; text-decoration: none; border-radius: 5px; }
        .nav a:hover { background: #5a6268; }
        .nav a.active { background: #007bff; }
        .scale-controls { margin: 15px 0; }
        .scale-controls label { display: inline-block; margin-right: 15px; }
        .scale-controls input[type="number"] { width: 80px; padding: 5px; }
        .result { margin-top: 30px; padding: 20px; background: #f8f9fa; border-radius: 5px; }
    </style>
</head>
<body>
<div class="container">
    <div class="nav">
        <a href="/form" class="active">Process Image</a>
        <a href="/preview">Preview Polygons</a>
        <a href="/config">Config</a>
    </div>
    <h1>Upload & Process Image</h1>
    
    <form action="/process" method="post" enctype="multipart/form-data">
        <input type="text" name="target" value="%s" placeholder="Telegram Chat ID">
        <input type="text" name="token" value="%s" placeholder="Bot Token">
        
        <div class="scale-controls">
            <label>
                <input type="checkbox" name="day" value="1" %s> Дневное время
            </label>
            <label>
                Масштаб полигонов: <input type="number" name="polygon_scale" value="%.1f" step="0.1" min="0.1" max="5.0">
            </label>
            <label>
                Смещение X: <input type="number" name="offset_x" value="%.0f" step="1">
            </label>
            <label>
                Смещение Y: <input type="number" name="offset_y" value="%.0f" step="1">
            </label>
        </div>
        
        <input type="file" name="file" accept="image/*" required />
        <div style="margin-top: 20px; text-align: center;">
            <input type="submit" value="Process & Send to Telegram" onclick="setFormAction('/process')" />
            <input type="submit" value="View Parking Algorithm" onclick="setFormAction('/view_parking_algorithm')" class="algorithm" />
        </div>
    </form>
    
    <div class="result">
        %s
    </div>
    
    <script>
        function setFormAction(action) {
            document.querySelector('form').action = action;
        }
    </script>
</div>
</body>
</html>
	`, data.Target, data.Token,
		func() string {
			if data.Day {
				return "checked"
			} else {
				return ""
			}
		}(),
		data.PolygonScale, data.OffsetX, data.OffsetY, result)

	w.Write([]byte(html))
}

// Новый обработчик для кнопки "Просмотр работы алгоритма"
func viewParkingAlgorithm(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	fmt.Printf("=== Processing parking algorithm view request ===\n")
	fmt.Printf("Method: %s, URL: %s\n", r.Method, r.URL.Path)
	fmt.Printf("Headers: %+v\n", r.Header)
	fmt.Println("Processing parking algorithm view...")

	polygons := appConfig.GetPolygons()
	if len(polygons) == 0 {
		showFormWithResult(w, r, `<div style="color: red;">No polygons available</div>`)
		return
	}

	// Получаем параметры трансформации полигонов
	polygonScale := 1.0
	offsetX := 0.0
	offsetY := 0.0

	if scale := r.FormValue("polygon_scale"); scale != "" {
		if s, err := strconv.ParseFloat(scale, 64); err == nil {
			polygonScale = s
		}
	}
	if offX := r.FormValue("offset_x"); offX != "" {
		if x, err := strconv.ParseFloat(offX, 64); err == nil {
			offsetX = x
		}
	}
	if offY := r.FormValue("offset_y"); offY != "" {
		if y, err := strconv.ParseFloat(offY, 64); err == nil {
			offsetY = y
		}
	}

	file, filename, err := r.FormFile("file")
	if err != nil {
		showFormWithResult(w, r, fmt.Sprintf(`<div style="color: red;">Failed to get file: %v</div>`, err))
		return
	}
	defer file.Close()

	img, err := decodeImage(file)
	if err != nil {
		showFormWithResult(w, r, fmt.Sprintf(`<div style="color: red;">Failed to decode image: %v</div>`, err))
		return
	}

	fmt.Printf("Processing parking algorithm for image: %s, size: %dx%d\n",
		filename.Filename, img.Bounds().Dx(), img.Bounds().Dy())

	isDay := r.FormValue("day") == "1"
	params := appConfig.ToProcessingParams(isDay)
	params.PolygonScale = polygonScale
	params.OffsetX = offsetX
	params.OffsetY = offsetY

	processedImg, err := analyzeParkingSpaces(img, polygons, params, isDay)
	if err != nil {
		showFormWithResult(w, r, fmt.Sprintf(`<div style="color: red;">Failed to analyze parking spaces: %v</div>`, err))
		return
	}

	processingTime := time.Since(start)
	fmt.Printf("Parking algorithm processing took %s\n", processingTime)

	// Конвертируем результат в base64 для отображения в HTML
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, processedImg, &jpeg.Options{Quality: 85})
	if err != nil {
		showFormWithResult(w, r, fmt.Sprintf(`<div style="color: red;">Failed to encode result: %v</div>`, err))
		return
	}

	// Кодируем в base64
	encoded := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	// Показываем результат
	result := fmt.Sprintf(`
		<h3>Результат анализа парковочных мест:</h3>
		<p><strong>Время обработки:</strong> %s</p>
		<p><strong>Найдено полигонов:</strong> %d</p>
		<p><strong>Настройки:</strong> Масштаб %.2f, Смещение (%.0f, %.0f), %s</p>
		<img src="%s" alt="Parking algorithm result" style="max-width: 100%%; height: auto;" />
	`, processingTime, len(polygons), polygonScale, offsetX, offsetY,
		func() string {
			if isDay {
				return "Дневной режим"
			} else {
				return "Ночной режим"
			}
		}(), encoded)

	showFormWithResult(w, r, result)
}
