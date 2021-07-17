package main

import (
	"bytes"
	"crypto/md5"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
	"runtime"
	"time"
)

var grayLayer image.Image

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	image.RegisterFormat(defaultMimeType, defaultMimeType, png.Decode, png.DecodeConfig)
}

// Pair ...
type Pair struct {
	Original image.Image
	Compared image.Image
}

var compared []image.Image

// var dotsLayer image.Image

func main() {

	start := time.Now()

	checkArgs()

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

	image1, _, err := image.DecodeConfig(img1)
	if err != nil {
		log.Println(err)
		return
	}

	image2, _, err := image.DecodeConfig(img2)
	if err != nil {
		log.Println(err)
		return
	}

	image1W := image1.Width
	image1H := image1.Height

	image2W := image2.Width
	image2H := image2.Height

	if !checkImgSizes(image1H, image2H, image1W, image2W) {
		log.Fatal("images must be the same size")
	}

	yIter := image1H / sizeQuad
	xIter := image1W / sizeQuad

	yMod := image1H % sizeQuad
	xMod := image1W % sizeQuad

	if yMod > 0 {
		yIter++
	}

	if xMod > 0 {
		xIter++
	}

	x0 := 0
	y0 := 0

	x1 := sizeQuad
	y1 := sizeQuad

	img1.Seek(0, 0)
	img2.Seek(0, 0)

	imgOne, _, err := image.Decode(img1)
	imgTwo, _, err := image.Decode(img2)

	chanPairs := make(chan Pair, xIter*yIter) // store Pairs for compare

	for y := 0; y < yIter; y++ {
		for x := 0; x < xIter; x++ {

			if x == xIter && xMod != 0 {
				x1 -= sizeQuad + xMod
			}

			if y == yIter && yMod != 0 {
				y1 -= sizeQuad + yMod
			}

			imgPart1 := SubImage(imgOne, x0, y0, x1, y1)
			imgPart2 := SubImage(imgTwo, x0, y0, x1, y1)

			bufImage1 := new(bytes.Buffer)
			bufImage2 := new(bytes.Buffer)

			errImage1 := png.Encode(bufImage1, imgPart1)
			errImage2 := png.Encode(bufImage2, imgPart2)

			if errImage1 != nil || errImage2 != nil {
				log.Fatalf("errImage1: = %v, errImage2 = %v", errImage1, errImage2)
			}

			if md5.Sum(bufImage1.Bytes()) == md5.Sum(bufImage2.Bytes()) {
				compared = append(compared, imgPart1)
			} else {
				chanPairs <- Pair{
					Original: imgPart1,
					Compared: imgPart2,
				}
			}

			x0 += sizeQuad
			x1 += sizeQuad
		}

		x0 = 0
		x1 = sizeQuad

		y0 += sizeQuad
		y1 += sizeQuad
	}

	// core.go
	parallelProcessDifferences(chanPairs, image1W, image1H)

	resultImage := image.NewRGBA(image.Rect(0, 0, image1W, image1H))
	clr2, _ := ToRGBA("FFFFFF")
	draw.Draw(resultImage, resultImage.Bounds(), image.NewUniform(clr2), image.Point{}, draw.Src)

	for idx := range compared {
		draw.Draw(resultImage, resultImage.Bounds(), compared[idx], image.Point{0, 0}, draw.Src)
	}

	// TODO: experiments with direct pixel processing
	/*
		 redRect := image.Rect(60, 80, 120, 160)
		 myred := color.RGBA{250, 0, 0, 150}

		 grayLayer := Grayscale(resultImage)
		 tens := toTensor(imgOne)
		 greyScale(&tens)
		 grayLayer := toImage(tens)

		draw.Draw(grayLayer.(draw.Image), redRect, &image.Uniform{myred}, image.Point{}, draw.Src)
	*/

	f, err := os.Create(resultImagePath)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()

	// err = png.Encode(f, grayLayer)
	err = png.Encode(f, resultImage)

	elapsed := time.Since(start)
	log.Printf("Diff took %s", elapsed)
}
