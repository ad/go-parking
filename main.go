package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"gocv.io/x/gocv"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("[source] [destination]")
		return
	}

	source := os.Args[1]
	dest := os.Args[2]

	img := gocv.IMRead(source, gocv.IMReadColor)
	if img.Empty() {
		fmt.Println("Error opening image")
		return
	}
	defer img.Close()

	rotationMatrix := gocv.GetRotationMatrix2D(image.Point{img.Cols() / 2, img.Rows() / 2}, 10, 0.9)
	gocv.WarpAffine(img, &img, rotationMatrix, image.Point{img.Cols(), img.Rows()})
	gocv.WarpAffine(img, &img, rotationMatrix, image.Point{img.Cols(), img.Rows()})

	edges := gocv.NewMatWithSize(img.Cols(), img.Rows(), gocv.MatTypeCV8S)
	gocv.Canny(img, &edges, 0, 512) // night
	// gocv.Canny(img, &edges, 196, 512) // day
	defer edges.Close()

	gocv.Threshold(edges, &edges, 254, 255, gocv.ThresholdBinaryInv)
	// gocv.Threshold(edges, &edges, 127, 255, gocv.ThresholdBinaryInv) // day

	croppedMat := edges.Region(image.Rect(655, 351, 1635, 1296))
	defer croppedMat.Close()

	gocv.IMWrite("edges.jpg", croppedMat)

	outputImg := img.Region(image.Rect(655, 351, 1635, 1296))
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
		image.Rect(71, 882, 140, 920),
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
			// gocv.Rectangle(&outputImg, rect, color.RGBA{0, uint8(255), 0, 0}, 2)
			// Assuming you want to fill the rectangle
			pts := [][]image.Point{{{rect.Min.X, rect.Min.Y}, {rect.Max.X, rect.Min.Y}, {rect.Max.X, rect.Max.Y}, {rect.Min.X, rect.Max.Y}}}
			gocv.FillPoly(&outputImg, gocv.NewPointsVectorFromPoints(pts), color.RGBA{0, uint8(255), 0, 0})
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
			gocv.PutText(
				&outputImg,
				fmt.Sprintf("%d", i+1),
				image.Pt(
					rect.Min.X+(rect.Dx()/3),
					rect.Min.Y+(rect.Dy()/2),
				),
				gocv.FontHersheyPlain,
				0.8,
				color.RGBA{255, 0, 0, 0},
				2,
			)
			gocv.Rectangle(&outputImg, rect, color.RGBA{uint8(255), 0, 0, uint8(255)}, 1)
		}
	}

	gocv.IMWrite(dest, outputImg)

	window := gocv.NewWindow("Rectangle")
	defer window.Close()
	window.IMShow(croppedMat)
	rect := window.SelectROIs(croppedMat)
	fmt.Printf("Selected rect: %+v\n", rect)
	window.WaitKey(0)
}
