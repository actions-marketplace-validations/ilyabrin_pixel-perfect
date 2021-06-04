package main

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
)

/*
   1. ok - open image
   2. ok - slice into separate parts (regions)
   3.    - process each part with goroutine
   4.    - save result into new image
*/

func init() {
	image.RegisterFormat("png", "png", png.Decode, png.DecodeConfig)
}

func main() {
	img1, err := os.Open("./test.png")

	if err != nil {
		fmt.Println("test.png file not found!")
		os.Exit(1)
	}
	defer img1.Close()

	imgCfg, _, err := image.DecodeConfig(img1)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	width := imgCfg.Width
	height := imgCfg.Height

	log.Println("Width : ", width)
	log.Println("Height : ", height)

	img1.Seek(0, 0)

	img, _, err := image.Decode(img1)

	x0 := 0
	y0 := 0
	x1 := 200
	y1 := 200

	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			// todo: send to chan with parts
			img_part := img.(interface {
				SubImage(r image.Rectangle) image.Image
			}).SubImage(image.Rect(x0, y0, x1, y1))

			fmt.Println(x0, y0, x1, y1)

			f, err := os.Create("./parts/part_" + fmt.Sprint(x) + "_" + fmt.Sprint(y) + ".png")
			if err != nil {
				log.Println(err)
			}
			defer f.Close()

			err = png.Encode(f, img_part)
			if err != nil {
				log.Println(err)
			}

			x0 += 200
			x1 += 200
		}

		x0 = 0
		x1 = 200
		y0 += 200
		y1 += 200

	}

	// enc := png.Encoder{
	// 	CompressionLevel: png.BestSpeed,
	// }
	// for y := 0; y < height; y++ {
	// 	for x := 0; x < width; x++ {
	// 		r, g, b, a := img.At(x, y).RGBA()
	// 		fmt.Printf("[X : %d Y : %v] R : %v, G : %v, B : %v, A : %v  \n", x, y, r, g, b, a)
	// 	}
	// }

	// TODO: diff("test_1.png", "test_2.png")

}

func diff(path_img1, path_img2 string) {
	img1, err := os.Open(path_img1)

	if err != nil {
		fmt.Println("file not found!")
		os.Exit(1)
	}
	defer img1.Close()

	img2, err := os.Open(path_img2)

	if err != nil {
		fmt.Println("file not found!")
		os.Exit(1)
	}
	defer img2.Close()

	imgCfg1, _, err := image.DecodeConfig(img1)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	w1 := imgCfg1.Width
	h1 := imgCfg1.Height

	println("-----")
	fmt.Println(w1, h1)

	// for y := 0; y < h1; y++ {
	// 	for x := 0; x < w1; x++ {
	// 		r, g, b, a := imgCfg1.At(x, y).RGBA()
	// 		fmt.Printf("[X : %d Y : %v] R : %v, G : %v, B : %v, A : %v  \n", x, y, r, g, b, a)
	// 	}
	// }

}
