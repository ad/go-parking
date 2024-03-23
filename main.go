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

	"github.com/fogleman/gg"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/goregular"
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

	imgGG := gg.NewContextForRGBA(imgRGBA)
	imgGG.SetLineWidth(2)

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

	utils.ParallelForEachPixel(grayscaleImg.Bounds().Size(), func(x int, y int) {
		if x < int(min.X*resizeScale) || x > int(max.X*resizeScale) || y < int(min.Y*resizeScale) || y > int(max.Y*resizeScale) {
			return
		}
		for _, polygon := range polygons {
			point := poly.XY{X: float64(x) / resizeScale, Y: float64(y) / resizeScale}
			if point.In(*polygon) {
				pixelGray := imgEdges.At(x, y)
				if pixelGray != emptyPixel {
					polygon.IncNonZero()
				} else {
					polygon.IncZero()
				}
			}
		}
	})

	font, _ := truetype.Parse(goregular.TTF)
	face := truetype.NewFace(font, &truetype.Options{Size: 16})
	imgGG.SetFontFace(face)

	for _, polygon := range polygons {
		if polygon.Zero != 0 {
			center := polygon.Center()
			percentage := 100 - float64(polygon.NonZero)/float64(polygon.Zero)*100

			// fmt.Println(i, "Percentage:", percentage, "%")
			if percentage > tresholdEmpty {
				col := color.RGBA{0, 255, 0, 255}
				DrawPolygon(imgGG, polygon, col, 3)
			}

			if percentage != 100 {
				DrawStrokeText(imgGG, fmt.Sprintf("%.1f", percentage), center.X, center.Y, color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}, 2)
			}
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

	imgRGBA = imgGG.Image().(*image.RGBA)

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
