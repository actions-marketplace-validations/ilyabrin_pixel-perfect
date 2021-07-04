package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"strconv"
)

/*
   1. ok - open image
   2. ok - slice into separate parts (regions)
   3.    - process each part with goroutine
   4. ok - save result into new image
*/

// todo: use in later
const (
	MAX_IMAGE_SIZE   = "100mb"
	MAX_IMAGE_HEIGHT = "8129px"
	MAX_IMAGE_WIDTH  = "8129px"
	MIN_IMAGE_HEIGHT = "1px"
	MIN_IMAGE_WIDTH  = "1px"

	QUAD_SIZE = 100 // 100px
)

func init() {
	image.RegisterFormat("png", "png", png.Decode, png.DecodeConfig)
}

var imageParts1, imageParts2 []image.Image

func main() {

	numArgs := len(os.Args[1:])
	if numArgs != 2 {
		log.Fatal("two args only error message")
	}

	arg1 := os.Args[1]
	arg2 := os.Args[2]

	if _, err := os.Stat(arg1); os.IsNotExist(err) {
		log.Fatalf("File %s is not exists", arg1)
	}

	if _, err := os.Stat(arg2); os.IsNotExist(err) {
		log.Fatalf("File %s is not exists", arg2)
	}

	img1, _ := os.Open(arg1)
	img2, _ := os.Open(arg2)

	defer img1.Close()
	defer img2.Close()

	img1_Cfg, _, err := image.DecodeConfig(img1)
	if err != nil {
		log.Println(err)
		return
	}

	img2_Cfg, _, err := image.DecodeConfig(img2)
	if err != nil {
		log.Println(err)
		return
	}

	img1_width := img1_Cfg.Width
	img1_height := img1_Cfg.Height

	img2_width := img2_Cfg.Width
	img2_height := img2_Cfg.Height

	if !checkImgSizes(img1_height, img2_height, img1_width, img2_width) {
		log.Fatal("images must be the same size")
	}

	y_iter := img1_height / QUAD_SIZE
	x_iter := img1_width / QUAD_SIZE

	y_mod := img1_height % QUAD_SIZE
	x_mod := img1_width % QUAD_SIZE

	if y_mod > 0 {
		y_iter += 1
	}

	if x_mod > 0 {
		x_iter += 1
	}

	x0 := 0
	y0 := 0

	x1 := QUAD_SIZE
	y1 := QUAD_SIZE

	img1.Seek(0, 0)
	img2.Seek(0, 0)

	imgOne, _, err := image.Decode(img1)
	imgTwo, _, err := image.Decode(img2)

	imageParts1 := make([]image.Image, 0, x_iter*y_iter)
	imageParts2 := make([]image.Image, 0, x_iter*y_iter)

	for y := 0; y < y_iter; y++ {
		for x := 0; x < x_iter; x++ {

			// TODO: send to chan with parts

			if x == x_iter && x_mod != 0 {
				x1 -= QUAD_SIZE + x_mod
			}

			if y == y_iter && y_mod != 0 {
				y1 -= QUAD_SIZE + y_mod
			}

			// (O_o) rewrite ASAP!
			img_part1 := imgOne.(interface {
				SubImage(r image.Rectangle) image.Image
			}).SubImage(image.Rect(x0, y0, x1, y1))
			imageParts1 = append(imageParts1, img_part1)

			img_part2 := imgTwo.(interface {
				SubImage(r image.Rectangle) image.Image
			}).SubImage(image.Rect(x0, y0, x1, y1))
			imageParts2 = append(imageParts2, img_part2)

			x0 += QUAD_SIZE
			x1 += QUAD_SIZE
		}

		x0 = 0
		x1 = QUAD_SIZE
		y0 += QUAD_SIZE
		y1 += QUAD_SIZE
	}

	newImage := image.NewRGBA(image.Rect(0, 0, img1_width, img1_height))
	clr, _ := ToRGBA("FFFFFF")

	draw.Draw(newImage, newImage.Bounds(), image.NewUniform(clr), image.Point{}, draw.Src)

	for idx := range imageParts1 {
		// imageParts1[idx], err = diff(imageParts1[idx], imageParts2[idx])
		img, err := diff(imageParts1[idx], imageParts2[idx])
		if err != nil {
			log.Fatal(err)
		}
		draw.Draw(newImage, newImage.Bounds(), img, image.Point{0, 0}, draw.Src)
		// draw.Draw(newImage, newImage.Bounds(), imageParts1[idx], image.Point{0, 0}, draw.Src)
	}

	f, err := os.Create("./parts/result.png")
	if err != nil {
		log.Println(err)
	}
	defer f.Close()

	err = png.Encode(f, newImage)
}

// pixel-by-pixel image parts comparison
func diff(img1, img2 image.Image) (image.Image, error) {

	bounds := img1.Bounds()

	w := bounds.Max.X
	h := bounds.Max.Y

	// todo: night code concept
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {

			r1, g1, b1, _ := img1.At(x, y).RGBA()
			r2, g2, b2, _ := img2.At(x, y).RGBA()

			img1.(draw.Image).Set(x, y, color.GrayModel.Convert(img1.At(x, y)))

			if (r1 + g1 + b1) != (r2 + g2 + b2) {
				img1.(draw.Image).Set(x, y, color.RGBA{uint8(250), uint8(0), uint8(0), uint8(255)})
			}
		}
	}
	return img1, nil
}

func ToRGBA(h string) (color.RGBA, error) {
	rgb, err := hex2RGB(h)
	if err != nil {
		return color.RGBA{}, err
	}

	return color.RGBA{R: rgb.red, G: rgb.green, B: rgb.blue, A: 255}, nil
}

func hex2RGB(hex string) (rgb, error) {
	values, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return rgb{}, err
	}

	return rgb{
		red:   uint8(values >> 16),
		green: uint8((values >> 8) & 0xFF),
		blue:  uint8(values & 0xFF),
	}, nil
}

type rgb struct {
	red   uint8
	green uint8
	blue  uint8
}

// compare images by resolution sizes
func checkImgSizes(img1h, img2h, img1w, img2w int) bool {
	if img1h != img2h || img1w != img2w {
		return false
	}
	return true
}
