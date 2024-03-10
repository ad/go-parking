package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"

	"gocv.io/x/gocv"
)

func main() {
	mux := http.NewServeMux()

	// return form for uploading image
	mux.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("get form...")
		w.Write([]byte(`
		<html>
		<body>
		<form action="/process" method="post" enctype="multipart/form-data">
		<input type="text" name="target" placeholder="target">
		<input type="text" name="token" placeholder="token">
		<input type="checkbox" name="day" value="1"> is day
		<input type="checkbox" name="update" value="1"> update
		<input type="file" name="file" />
		<input type="submit" value="Upload" />
		</form>
		</body>	
		</html>
		`))
	})

	mux.HandleFunc("/process", processImage)

	fmt.Println("Server is running on localhost:9991")

	http.ListenAndServe("0.0.0.0:9991", mux)
}

func processImage(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Processing image...")

	isDay := true

	if r.FormValue("day") == "1" {
		isDay = true
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

	imgmat, err := gocv.ImageToMatRGB(img)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if imgmat.Empty() {
		http.Error(w, "Empty image", http.StatusBadRequest)
		return
	}
	defer imgmat.Close()

	rotationMatrix := gocv.GetRotationMatrix2D(image.Point{imgmat.Cols() / 2, imgmat.Rows() / 2}, 10, 0.9)
	gocv.WarpAffine(imgmat, &imgmat, rotationMatrix, image.Point{imgmat.Cols(), imgmat.Rows()})
	gocv.WarpAffine(imgmat, &imgmat, rotationMatrix, image.Point{imgmat.Cols(), imgmat.Rows()})

	edges := gocv.NewMatWithSize(imgmat.Cols(), imgmat.Rows(), gocv.MatTypeCV8S)
	if isDay {
		gocv.Canny(imgmat, &edges, 196, 512) // day
	} else {
		gocv.Canny(imgmat, &edges, 0, 512) // night
	}
	defer edges.Close()

	if isDay {
		gocv.Threshold(edges, &edges, 127, 255, gocv.ThresholdBinaryInv) // day
	} else {
		gocv.Threshold(edges, &edges, 254, 255, gocv.ThresholdBinaryInv)
	}

	croppedMat := edges.Region(image.Rect(655, 351, 1635, 1296))
	defer croppedMat.Close()

	// gocv.IMWrite("edges.jpg", croppedMat)

	outputImg := imgmat.Region(image.Rect(655, 351, 1635, 1296))
	defer outputImg.Close()

	checkRects := []image.Rectangle{
		image.Rect(195, 254, 224, 312), // 1
		image.Rect(241, 251, 270, 307),
		image.Rect(283, 251, 310, 309),
		image.Rect(323, 250, 351, 308),
		image.Rect(380, 276, 451, 314),
		image.Rect(334, 369, 402, 412),
		image.Rect(170, 411, 212, 475),
		image.Rect(213, 410, 244, 475),
		image.Rect(245, 410, 276, 482),
		image.Rect(500, 288, 584, 337), // 10
		image.Rect(510, 338, 580, 381),
		image.Rect(234, 583, 312, 618),
		image.Rect(338, 564, 415, 616),
		image.Rect(841, 63, 877, 120),
		image.Rect(847, 150, 882, 217),
		image.Rect(765, 240, 847, 277),
		image.Rect(773, 278, 851, 323),
		image.Rect(788, 319, 857, 369),
		image.Rect(789, 359, 861, 423),
		image.Rect(788, 415, 863, 479), // 20
		image.Rect(923, 57, 964, 126),
		image.Rect(924, 127, 965, 209),
		image.Rect(925, 223, 966, 290),
		image.Rect(926, 294, 967, 376),
		image.Rect(927, 381, 968, 462),
		image.Rect(526, 432, 595, 514),
		image.Rect(513, 386, 582, 448),
		image.Rect(559, 477, 625, 555),
		image.Rect(425, 660, 497, 727),
		image.Rect(208, 725, 248, 804), // 30
		image.Rect(205, 819, 240, 880),
		image.Rect(44, 786, 88, 866),
		image.Rect(333, 661, 420, 703),
		image.Rect(3, 785, 41, 864),
		image.Rect(850, 228, 885, 292),
		// image.Rect(671, 253, 754, 278),
		// image.Rect(671, 286, 766, 321),
		image.Rect(79, 878, 141, 919),
		image.Rect(189, 324, 245, 351),
		image.Rect(442, 393, 470, 483),
		image.Rect(431, 498, 470, 574),
		image.Rect(253, 665, 326, 717), // 40
		image.Rect(76, 653, 156, 700),
		image.Rect(667, 496, 702, 556),
		// image.Rect(71, 882, 140, 920),
		image.Rect(633, 497, 661, 556),
		image.Rect(135, 790, 180, 880),
		image.Rect(1, 878, 28, 939),
		// image.Rect(180, 355, 243, 385),
	}

	threshold := 94.0

	for i, rect := range checkRects {
		testRegion := croppedMat.Region(rect)
		defer testRegion.Close()
		// gocv.IMWrite(fmt.Sprintf("testRegion_%d.jpg", i), testRegion)
		emptyCount := gocv.CountNonZero(testRegion)

		emptyPercentage := float64(float64(emptyCount)/float64(testRegion.Total())) * 100
		fmt.Println(i+1, emptyCount, testRegion.Total(), emptyPercentage, "%")

		if emptyPercentage > threshold {
			gocv.Rectangle(&outputImg, rect, color.RGBA{0, uint8(255), 0, 0}, 2)
			// Assuming you want to fill the rectangle
			// pts := [][]image.Point{{{rect.Min.X, rect.Min.Y}, {rect.Max.X, rect.Min.Y}, {rect.Max.X, rect.Max.Y}, {rect.Min.X, rect.Max.Y}}}
			// gocv.FillPoly(&outputImg, gocv.NewPointsVectorFromPoints(pts), color.RGBA{0, uint8(255), 0, 0})
			gocv.PutText(
				&outputImg,
				fmt.Sprintf("%d", i+1),
				image.Pt(
					rect.Min.X+(rect.Dx()/3),
					rect.Min.Y+(rect.Dy()/2),
				),
				gocv.FontHersheyPlain,
				0.8,
				color.RGBA{0, 0, 0, 0},
				2,
			)
		} else {
			// gocv.PutText(
			// 	&outputImg,
			// 	fmt.Sprintf("%d", i+1),
			// 	image.Pt(
			// 		rect.Min.X+(rect.Dx()/3),
			// 		rect.Min.Y+(rect.Dy()/2),
			// 	),
			// 	gocv.FontHersheyPlain,
			// 	0.8,
			// 	color.RGBA{255, 0, 0, 0},
			// 	2,
			// )
			// gocv.Rectangle(&outputImg, rect, color.RGBA{uint8(255), 0, 0, uint8(255)}, 1)
		}
	}

	output, err := outputImg.ToImage()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chatID, err := strconv.ParseInt(r.FormValue("target"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isUpdate := true

	if r.FormValue("update") == "1" {
		isUpdate = true
	}

	token := r.FormValue("token")

	if isUpdate {
		messageID := r.FormValue("message_id")
		threadID := r.FormValue("thread_id")

		sendImageUpdateToTelegram(output, chatID, messageID, threadID, token)

		return
	}

	sendImageTotelegram(output, chatID, token)
}

func sendImageTotelegram(img image.Image, chatID int64, botToken string) {
	file, err := os.Create("output.jpg")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	err = jpeg.Encode(file, img, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto?chat_id=%d", botToken, chatID)

	resp, err := upload(url, img)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(string(body))
}

func sendImageUpdateToTelegram(img image.Image, chatID int64, messageID, threadID, botToken string) {
	file, err := os.Create("output.jpg")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	err = jpeg.Encode(file, img, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(string(body))
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
	fmt.Println(resp.Header)
	fmt.Println(resp.Body)

	return resp, nil
}
