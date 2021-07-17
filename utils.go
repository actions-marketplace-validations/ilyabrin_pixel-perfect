package main

import (
	"image"
	"image/color"
	"image/draw"
	"log"
	"os"
	"strconv"
	"sync"
)

type rgb struct {
	red   uint8
	green uint8
	blue  uint8
}

// convert #ffffff to RGB form
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

// ToRGBA return the RGBA color from hex code
func ToRGBA(h string, opacity ...uint8) (color.RGBA, error) {

	rgb, err := hex2RGB(h)

	if err != nil {
		return color.RGBA{}, err
	}

	if len(opacity) != 0 {
		return color.RGBA{R: rgb.red, G: rgb.green, B: rgb.blue, A: opacity[0]}, nil
	}

	return color.RGBA{R: rgb.red, G: rgb.green, B: rgb.blue, A: 255}, nil

}

// SubImage returns image rect with given coords
func SubImage(img image.Image, x1, y1, x2, y2 int) image.Image {
	return img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(x1, y1, x2, y2))
}

// compare images by resolution sizes
func checkImgSizes(img1h, img2h, img1w, img2w int) bool {
	if img1h != img2h || img1w != img2w {
		return false
	}
	return true
}

// TODO: add border thickness
func drawRectangle(img draw.Image, rect image.Rectangle, color color.Color) draw.Image {

	min := rect.Min
	max := rect.Max

	for i := min.X; i < max.X; i++ {
		img.Set(i, min.Y, color)
		img.Set(i, max.Y, color)
	}

	for i := min.Y; i <= max.Y; i++ {
		img.Set(min.X, i, color)
		img.Set(max.X, i, color)
	}

	return img

}

// cheking the args of cli
func checkArgs() {

	numArgs := len(os.Args[1:])

	if numArgs != 2 {
		log.Fatal("two args only error message")
	}

}

// func combineLayers(base, gray image.Image) image.Image {
func combineLayers(base, gray image.Image) image.Image {

	out := image.NewRGBA(base.Bounds())

	draw.Draw(out, base.Bounds(), base, image.Point{0, 0}, draw.Src)  // base image
	draw.Draw(out, base.Bounds(), gray, image.Point{0, 0}, draw.Over) // gray alpha
	// draw.Draw(out, base.Bounds(), dots, image.Point{0, 0}, draw.Over) // red dots

	// color, _ := ToRGBA("FFFFFF")
	// drawRectangle(out, image.Rect(10, 20, 300, 400), color)

	return out
}

// combine colors instead of img.Set (which rewrites the value at place)
func combine(c1, c2 color.Color) color.Color {

	r, g, b, a := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()

	return color.RGBA{
		uint8((r + r2) >> 9), // div by 2 followed by ">> 8"  is ">> 9"
		uint8((g + g2) >> 9),
		uint8((b + b2) >> 9),
		uint8((a + a2) >> 9),
	}

}

// converts image.Image to two three two dimensional array
// TODO: add benchmark
func toTensor(img image.Image) [][]color.Color {

	size := img.Bounds().Size()
	var pixels [][]color.Color

	for i := 0; i < size.X; i++ {
		var y []color.Color
		for j := 0; j < size.Y; j++ {
			y = append(y, img.At(i, j))
		}
		pixels = append(pixels, y)
	}

	return pixels
}

// converts pixel matrix to image.Image
// TODO: add benchmark
func toImage(pixels [][]color.Color) image.Image {

	rect := image.Rect(0, 0, len(pixels), len(pixels[0]))
	nImg := image.NewRGBA(rect)

	for x := 0; x < len(pixels); x++ {
		for y := 0; y < len(pixels[0]); y++ {
			q := pixels[x]
			if q == nil {
				continue
			}
			p := pixels[x][y]
			if p == nil {
				continue
			}
			original, ok := color.RGBAModel.Convert(p).(color.RGBA)
			if ok {
				nImg.Set(x, y, original)
			}
		}
	}

	return nImg

}

// TODO: add benchmark
func greyScale(pixels *[][]color.Color) {
	ppixels := *pixels
	xLen := len(ppixels)
	yLen := len(ppixels[0])
	//create new image
	newImage := make([][]color.Color, xLen)
	for i := 0; i < len(newImage); i++ {
		newImage[i] = make([]color.Color, yLen)
	}
	// processing pixels in parallel
	wg := sync.WaitGroup{}
	for x := 0; x < xLen; x++ {
		for y := 0; y < yLen; y++ {
			wg.Add(1)
			go func(x, y int) {
				pixel := ppixels[x][y]
				originalColor, ok := color.RGBAModel.Convert(pixel).(color.RGBA)
				if !ok {
					log.Println("type conversion went wrong")
				}
				grey := uint8(float64(originalColor.R)*0.21 + float64(originalColor.G)*0.72 + float64(originalColor.B)*0.07)
				col := color.RGBA{
					grey,
					grey,
					grey,
					originalColor.A,
				}
				newImage[x][y] = col
				wg.Done()
			}(x, y)
		}
	}
	wg.Wait()
	*pixels = newImage
}
