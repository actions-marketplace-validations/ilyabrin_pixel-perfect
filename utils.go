package main

import (
	"image"
	"image/color"
	"math"
	"strconv"
)

type rgb struct {
	red   uint8
	green uint8
	blue  uint8
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

func Grayscale(part image.Image) {
	grayScale := image.NewGray(image.Rectangle{image.Point{0, 0}, image.Point{QUAD_SIZE, QUAD_SIZE}})
	for x := 0; x < QUAD_SIZE; x++ {
		for y := 0; y < QUAD_SIZE; y++ {
			imageColor := part.At(x, y)
			rr, gg, bb, _ := imageColor.RGBA()
			r := math.Pow(float64(rr), 2.2)
			g := math.Pow(float64(gg), 2.2)
			b := math.Pow(float64(bb), 2.2)
			m := math.Pow(0.2125*r+0.7154*g+0.0721*b, 1/2.2)
			Y := uint16(m + 0.5)
			grayColor := color.Gray{uint8(Y >> 8)}
			grayScale.Set(x, y, grayColor)
		}
	}
}

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
