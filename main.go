package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"

	"github.com/ernyoke/imger/edgedetection"
	"github.com/ernyoke/imger/effects"
	"github.com/ernyoke/imger/grayscale"
	"github.com/ernyoke/imger/resize"
	"github.com/ernyoke/imger/transform"
	"github.com/ernyoke/imger/utils"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/llgcode/draw2d/draw2dimg"
	"github.com/llgcode/draw2d/draw2dkit"
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

	// isDay := true

	// if r.FormValue("day") == "1" {
	// 	isDay = true
	// }

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

	// imgmat, err := gocv.ImageToMatRGB(img)
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return
	// }
	// if imgmat.Empty() {
	// 	http.Error(w, "Empty image", http.StatusBadRequest)
	// 	return
	// }
	// defer imgmat.Close()

	// rotationMatrix := gocv.GetRotationMatrix2D(image.Point{imgmat.Cols() / 2, imgmat.Rows() / 2}, 10, 0.9)
	// gocv.WarpAffine(imgmat, &imgmat, rotationMatrix, image.Point{imgmat.Cols(), imgmat.Rows()})
	// gocv.WarpAffine(imgmat, &imgmat, rotationMatrix, image.Point{imgmat.Cols(), imgmat.Rows()})

	// edges := gocv.NewMatWithSize(imgmat.Cols(), imgmat.Rows(), gocv.MatTypeCV8S)
	// if isDay {
	// 	gocv.Canny(imgmat, &edges, 196, 512) // day
	// } else {
	// 	gocv.Canny(imgmat, &edges, 0, 512) // night
	// }
	// defer edges.Close()

	// if isDay {
	// 	gocv.Threshold(edges, &edges, 127, 255, gocv.ThresholdBinaryInv) // day
	// } else {
	// 	gocv.Threshold(edges, &edges, 254, 255, gocv.ThresholdBinaryInv)
	// }

	// croppedMat := edges.Region(image.Rect(655, 351, 1635, 1296))
	// defer croppedMat.Close()

	// // gocv.IMWrite("edges.jpg", croppedMat)

	// outputImg := imgmat.Region(image.Rect(655, 351, 1635, 1296))
	// defer outputImg.Close()

	// checkRects := []image.Rectangle{
	// 	image.Rect(195, 254, 224, 312), // 1
	// 	image.Rect(241, 251, 270, 307),
	// 	image.Rect(283, 251, 310, 309),
	// 	image.Rect(323, 250, 351, 308),
	// 	image.Rect(380, 276, 451, 314),
	// 	image.Rect(334, 369, 402, 412),
	// 	image.Rect(170, 411, 212, 475),
	// 	image.Rect(213, 410, 244, 475),
	// 	image.Rect(245, 410, 276, 482),
	// 	image.Rect(500, 288, 584, 337), // 10
	// 	image.Rect(510, 338, 580, 381),
	// 	image.Rect(234, 583, 312, 618),
	// 	image.Rect(338, 564, 415, 616),
	// 	image.Rect(841, 63, 877, 120),
	// 	image.Rect(847, 150, 882, 217),
	// 	image.Rect(765, 240, 847, 277),
	// 	image.Rect(773, 278, 851, 323),
	// 	image.Rect(788, 319, 857, 369),
	// 	image.Rect(789, 359, 861, 423),
	// 	image.Rect(788, 415, 863, 479), // 20
	// 	image.Rect(923, 57, 964, 126),
	// 	image.Rect(924, 127, 965, 209),
	// 	image.Rect(925, 223, 966, 290),
	// 	image.Rect(926, 294, 967, 376),
	// 	image.Rect(927, 381, 968, 462),
	// 	image.Rect(526, 432, 595, 514),
	// 	image.Rect(513, 386, 582, 448),
	// 	image.Rect(559, 477, 625, 555),
	// 	image.Rect(425, 660, 497, 727),
	// 	image.Rect(208, 725, 248, 804), // 30
	// 	image.Rect(205, 819, 240, 880),
	// 	image.Rect(44, 786, 88, 866),
	// 	image.Rect(333, 661, 420, 703),
	// 	image.Rect(3, 785, 41, 864),
	// 	image.Rect(850, 228, 885, 292),
	// 	// image.Rect(671, 253, 754, 278),
	// 	// image.Rect(671, 286, 766, 321),
	// 	image.Rect(79, 878, 141, 919),
	// 	image.Rect(189, 324, 245, 351),
	// 	image.Rect(442, 393, 470, 483),
	// 	image.Rect(431, 498, 470, 574),
	// 	image.Rect(253, 665, 326, 717), // 40
	// 	image.Rect(76, 653, 156, 700),
	// 	image.Rect(667, 496, 702, 556),
	// 	// image.Rect(71, 882, 140, 920),
	// 	image.Rect(633, 497, 661, 556),
	// 	image.Rect(135, 790, 180, 880),
	// 	image.Rect(1, 878, 28, 939),
	// 	// image.Rect(180, 355, 243, 385),
	// }

	// threshold := 94.0

	// for i, rect := range checkRects {
	// 	testRegion := croppedMat.Region(rect)
	// 	defer testRegion.Close()
	// 	// gocv.IMWrite(fmt.Sprintf("testRegion_%d.jpg", i), testRegion)
	// 	emptyCount := gocv.CountNonZero(testRegion)

	// 	emptyPercentage := float64(float64(emptyCount)/float64(testRegion.Total())) * 100
	// 	fmt.Println(i+1, emptyCount, testRegion.Total(), emptyPercentage, "%")

	// 	if emptyPercentage > threshold {
	// 		gocv.Rectangle(&outputImg, rect, color.RGBA{0, uint8(255), 0, 0}, 2)
	// 		// Assuming you want to fill the rectangle
	// 		// pts := [][]image.Point{{{rect.Min.X, rect.Min.Y}, {rect.Max.X, rect.Min.Y}, {rect.Max.X, rect.Max.Y}, {rect.Min.X, rect.Max.Y}}}
	// 		// gocv.FillPoly(&outputImg, gocv.NewPointsVectorFromPoints(pts), color.RGBA{0, uint8(255), 0, 0})
	// 		gocv.PutText(
	// 			&outputImg,
	// 			fmt.Sprintf("%d", i+1),
	// 			image.Pt(
	// 				rect.Min.X+(rect.Dx()/3),
	// 				rect.Min.Y+(rect.Dy()/2),
	// 			),
	// 			gocv.FontHersheyPlain,
	// 			0.8,
	// 			color.RGBA{0, 0, 0, 0},
	// 			2,
	// 		)
	// 	} else {
	// 		// gocv.PutText(
	// 		// 	&outputImg,
	// 		// 	fmt.Sprintf("%d", i+1),
	// 		// 	image.Pt(
	// 		// 		rect.Min.X+(rect.Dx()/3),
	// 		// 		rect.Min.Y+(rect.Dy()/2),
	// 		// 	),
	// 		// 	gocv.FontHersheyPlain,
	// 		// 	0.8,
	// 		// 	color.RGBA{255, 0, 0, 0},
	// 		// 	2,
	// 		// )
	// 		// gocv.Rectangle(&outputImg, rect, color.RGBA{uint8(255), 0, 0, uint8(255)}, 1)
	// 	}
	// }

	// output, err := outputImg.ToImage()
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return
	// }

	// Rotate image
	imgRGBA, errRotate := transform.RotateRGBA(imgRGBA, 20, image.Point{img.Bounds().Dx() / 2, img.Bounds().Dy() / 2}, true)
	if errRotate != nil {
		log.Fatal("Could not rotate image", errRotate)
	}

	imgResized, errResize := resize.ResizeRGBA(imgRGBA, 0.8, 0.8, resize.InterNearest)
	if errResize != nil {
		log.Fatal("Could not resize image", errResize)
	}

	grayscaleImg := grayscale.Grayscale(imgResized)

	// Edge detection
	imgEdges, errCanny := edgedetection.CannyGray(grayscaleImg, 1, 192, 1)
	// imgEdges, errCanny := edgedetection.CannyGray(grayscaleImg, 32, 255, 1)
	// imgEdges, errCanny := edgedetection.SobelGray(grayscaleImg, 1)
	// imgEdges, errCanny := edgedetection.LaplacianGray(grayscaleImg, padding.BorderReplicate, edgedetection.K8)
	if errCanny != nil {
		log.Fatal("Could not detect edges", errCanny)
	}

	// Invert image
	imgEdges = effects.InvertGray(imgEdges)

	cropRect := image.Rect(550, 514, 1524, 1454)

	// Crop
	croppedEdges := imgEdges.SubImage(cropRect)

	// Crop
	croppedImg := imgResized.SubImage(cropRect)

	b = croppedImg.Bounds()
	output := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(output, output.Bounds(), croppedImg, b.Min, draw.Src)

	c := croppedEdges.Bounds()
	edgesImg := image.NewRGBA(image.Rect(0, 0, c.Dx(), c.Dy()))
	draw.Draw(edgesImg, edgesImg.Bounds(), croppedEdges, c.Min, draw.Src)

	checkRects := []image.Rectangle{
		image.Rect(195, 248, 224, 308), // 1
		image.Rect(241, 248, 270, 308),
		image.Rect(283, 248, 310, 308),
		image.Rect(323, 248, 351, 308),
		image.Rect(373, 268, 447, 306),
		image.Rect(334, 369, 402, 412),
		image.Rect(170, 400, 208, 475),
		image.Rect(213, 400, 244, 475),
		image.Rect(250, 400, 278, 475), //???
		image.Rect(500, 288, 584, 337), // 10
		image.Rect(510, 338, 580, 381),
		image.Rect(234, 583, 312, 618),
		image.Rect(338, 564, 415, 616),
		image.Rect(839, 55, 877, 128),
		image.Rect(840, 140, 878, 217),
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
		image.Rect(510, 384, 595, 448),
		image.Rect(559, 477, 625, 555),
		image.Rect(425, 660, 497, 710),
		image.Rect(206, 732, 246, 812), // 30
		image.Rect(202, 822, 242, 882),
		image.Rect(44, 786, 88, 866),
		image.Rect(333, 661, 420, 703),
		image.Rect(3, 785, 41, 864),
		image.Rect(79, 878, 141, 919),
		image.Rect(189, 324, 245, 351),
		image.Rect(442, 393, 470, 483),
		image.Rect(431, 498, 470, 574),
		image.Rect(230, 665, 320, 715), // 40
		image.Rect(76, 653, 156, 700),
		image.Rect(667, 496, 702, 548),
		// image.Rect(71, 882, 140, 920),
		image.Rect(625, 500, 650, 547),
		image.Rect(135, 783, 171, 870),
		image.Rect(1, 878, 28, 939),
		// image.Rect(180, 355, 243, 385),
	}

	threshold := 96.50
	// threshold := 30.0

	gc := draw2dimg.NewGraphicContext(output)

	emptyPixel := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	for i, rect := range checkRects {
		rectImage := edgesImg.SubImage(rect)

		// if i == 30 {
		// 	rectFile, err := os.Create("test_rect.png")
		// 	if err != nil {
		// 		panic(err.Error())
		// 	}

		// 	png.Encode(rectFile, rectImage)

		// 	rectFile.Close()
		// }

		emptyPixelCount := 0.0
		notEmptyPixelCount := 0.0

		utils.ParallelForEachPixel(rectImage.Bounds().Size(), func(x int, y int) {
			pixel := rectImage.At(rectImage.Bounds().Min.X+x, rectImage.Bounds().Min.Y+y)
			if pixel != emptyPixel {
				// fmt.Printf("%dx%d, %#v\n", x, y, pixel)
				notEmptyPixelCount++
			} else {
				emptyPixelCount++
			}
		})

		emptyPercent := 100 - ((notEmptyPixelCount / emptyPixelCount) * 100)
		fmt.Println(i, emptyPixelCount, notEmptyPixelCount, emptyPercent)

		if emptyPercent > threshold {
			gc.SetStrokeColor(color.RGBA{0, 255, 0, 255})
			draw2dkit.Rectangle(gc, float64(rect.Min.X), float64(rect.Min.Y), float64(rect.Max.X), float64(rect.Max.Y))
			gc.Stroke()

			addLabel(output, rect.Min.X, rect.Max.Y, fmt.Sprintf("%d %.0f", i, emptyPercent), color.RGBA{0, 255, 0, 255})
		} else {
			gc.SetStrokeColor(color.RGBA{255, 0, 0, 255})
			draw2dkit.Rectangle(gc, float64(rect.Min.X), float64(rect.Min.Y), float64(rect.Max.X), float64(rect.Max.Y))
			gc.Stroke()
			addLabel(output, rect.Min.X, rect.Max.Y, fmt.Sprintf("%d %.0f", i, emptyPercent), color.RGBA{255, 0, 0, 255})
		}
	}

	// draw2dimg.SaveToPngFile("result.png", output)
	// draw2dimg.SaveToPngFile("edgesImg.png", edgesImg)

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

func addLabel(img *image.RGBA, x, y int, label string, col color.RGBA) {
	point := fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(label)
}
