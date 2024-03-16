package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/ad/go-parking/poly"
	"github.com/ernyoke/imger/edgedetection"
	"github.com/ernyoke/imger/effects"
	"github.com/ernyoke/imger/grayscale"
	"github.com/ernyoke/imger/resize"
	"github.com/ernyoke/imger/utils"
)

var formTemplate = `
<html>
<body>
<form action="/process" method="post" enctype="multipart/form-data">
<input type="text" name="target" value="${target}" placeholder="target">
<input type="text" name="token" value="${token}" placeholder="token">
<input type="checkbox" name="day" value="${day}" checked> is day
<input type="file" name="file" />
<input type="submit" value="Upload" />
</form>
</body>	
</html>
`

func main() {
	mux := http.NewServeMux()

	// return form for uploading image
	mux.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("get form...")
		polygon := polygons[0]
		min, max := polygon.MinMax()
		fmt.Println("minMax:", min, max)

		w.Write([]byte(formatForm(r)))
	})

	mux.HandleFunc("/process", processImage)

	fmt.Println("Server is running on localhost:9991")

	http.ListenAndServe("0.0.0.0:9991", mux)
}

func processImage(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(formatForm(r)))

	resizeScale := 0.5

	start := time.Now()
	fmt.Println("Processing image...")

	isDay := true

	if r.FormValue("day") == "1" {
		isDay = true
	}

	tresholdEmpty := 94.0
	tresholdEdges := 128.0
	if isDay {
		tresholdEmpty = 96.0
		tresholdEdges = 192.0
	}

	// get file from form
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// convert file to image
	img, _, err := image.Decode(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	b := img.Bounds()
	imgRGBA := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(imgRGBA, imgRGBA.Bounds(), img, b.Min, draw.Src)

	grayscaleImg := grayscale.Grayscale(imgRGBA)
	grayscaleImg, err = effects.SharpenGray(grayscaleImg)
	if err != nil {
		fmt.Printf("could not sharpen image: %s", err)

		return
	}

	if resizeScale != 1.0 {
		// Resize image to half size for faster processing
		grayscaleImg, err = resize.ResizeGray(grayscaleImg, 0.5, 0.5, resize.InterNearest)
		if err != nil {
			fmt.Printf("could not resize image: %s", err)

			return
		}
	}

	// Edge detection
	imgEdges, errCanny := edgedetection.CannyGray(grayscaleImg, 1, tresholdEdges, 1)

	if errCanny != nil {
		fmt.Printf("could not detect edges: %s", errCanny)
		return
	}

	// Invert image
	imgEdges = effects.InvertGray(imgEdges)

	emptyPixel := color.Gray{Y: 0xff}

	min, max := poly.MinMaxMany(polygons)
	// minMaxPoly := &poly.Poly{
	// 	XY: []poly.XY{
	// 		{X: min.X, Y: min.Y},
	// 		{X: max.X, Y: min.Y},
	// 		{X: max.X, Y: max.Y},
	// 		{X: min.X, Y: max.Y},
	// 	},
	// }
	// fmt.Println("minMaxPoly:", min, max)
	// DrawPolygon(imgRGBA, minMaxPoly, color.RGBA{255, 0, 255, 255})

	utils.ParallelForEachPixel(grayscaleImg.Bounds().Size(), func(x int, y int) {
		if x < int(min.X*resizeScale) || x > int(max.X*resizeScale) || y < int(min.Y*resizeScale) || y > int(max.Y*resizeScale) {
			// imgRGBA.Set(int(float64(x)/resizeScale), int(float64(y)/resizeScale), color.RGBA{255, 0, 255, 255})
			// fmt.Println("out of bounds:", x, y)
			return
		}
		for _, polygon := range polygons {
			point := poly.XY{X: float64(x) / resizeScale, Y: float64(y) / resizeScale}
			if point.In(*polygon) {
				// r, g, _, _ := img.At(x, y).RGBA()
				// img.Set(x, y, color.RGBA{uint8(r) + 10, uint8(g) + 10, 255, 255})

				pixelGray := imgEdges.At(x, y)
				if pixelGray != emptyPixel {
					// img.Set(x, y, color.RGBA{255, 0, 0, 255})
					polygon.IncNonZero()
				} else {
					polygon.IncZero()
				}
			}
		}
	})

	for _, polygon := range polygons {
		if polygon.Zero != 0 {
			center := polygon.Center()
			percentage := 100 - float64(polygon.NonZero)/float64(polygon.Zero)*100

			// fmt.Println(i, "Percentage:", percentage, "%")
			if percentage > tresholdEmpty {
				// if percentage > 33 {
				DrawPolygon(imgRGBA, polygon, color.RGBA{0, 255, 0, 255})
				DrawLabel(imgRGBA, int(center.X), int(center.Y), fmt.Sprintf("%.1f", percentage), color.RGBA{0, 255, 0, 255})
			} else {
				DrawLabel(imgRGBA, int(center.X), int(center.Y), fmt.Sprintf("%.1f", percentage), color.RGBA{255, 0, 0, 255})
				// DrawPolygon(img, polygon, color.RGBA{255, 0, 0, 255})
			}

			// DrawLabel(imgRGBA, int(center.X), int(center.Y), fmt.Sprintf("%d %.1f", i, percentage), color.RGBA{255, 0, 255, 255})

		}
	}

	fmt.Printf("took %s\n", time.Since(start))

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

	_, err = ioutil.ReadAll(resp.Body)
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

	_, err = ioutil.ReadAll(resp.Body)
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

	req.Body = ioutil.NopCloser(&b)

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

// func addLabel(img *image.RGBA, x, y int, label string, col color.RGBA) {
// 	point := fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)}

// 	d := &font.Drawer{
// 		Dst:  img,
// 		Src:  image.NewUniform(col),
// 		Face: basicfont.Face7x13,
// 		Dot:  point,
// 	}
// 	d.DrawString(label)
// }

func formatForm(r *http.Request) string {
	// Таблица значений переменных
	varTable := map[string]string{
		"target": r.FormValue("target"),
		"token":  r.FormValue("token"),
		"day":    r.FormValue("day"),
	}

	// Функция замены, подставляет значение переменной из таблицы varTable
	substitutor := func(match string) string {
		// match - значение вида `${var_name}`
		// сначала извлечём var_name
		varName := match[2 : len(match)-1]
		// Теперь получим значение из таблицы
		value, ok := varTable[varName]
		if !ok {
			// один из вариантов обработки отсутствующего значения - вернуть пустую строку
			value = ""
		}
		return value
	}

	// Регулярное выражение для поиска строк вида
	// 'начинается с ${, затем любые символы кроме { и }
	// в любом количестве, заканчивается }'
	re := regexp.MustCompile(`\${[^{}]*}`)

	return re.ReplaceAllStringFunc(formTemplate, substitutor)
}
