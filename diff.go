package main

import (
	"bytes"
	"crypto/md5"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"runtime"
	"sync"
	"time"
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

	RESULT_IMAGE_PATH = "./parts/result.png"

	QUAD_SIZE = 100 // 100px
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	image.RegisterFormat("png", "png", png.Decode, png.DecodeConfig)
}

// var imageParts1, imageParts2, compared []image.Image
var compared []image.Image

type Pair struct {
	Original image.Image
	Compared image.Image
}

// pair send to compare
// compared receives result part (will rendered at final)
func parallelProcessDifferences(parts chan Pair, w, h int) {
	// помещаем в канал parts пару Pair{orig, comp}
	// затем в горутине сравниваем пары и пишем результат в compared
	// после всех проверок и сравнений рендерим все части из compared

	var wg sync.WaitGroup

	len_parts := len(parts)

	// TODO: должно быть не так, цикл заменить на горутину и канал(ы)
	// правильно так: diff проверяет совпадение и помещает в канал результат
	// как только результат появится в канале - другая горутина забирет его в компаред
	// если все пары обработаны - рендерим картинку и завершаем работу
	for x := 0; x < len_parts; x++ {
		wg.Add(1) // TODO: должны параллельно сразу все пары влететь, а не по одной
		part := <-parts
		go diff(part.Original, part.Compared, &wg)
	}
	wg.Wait()
	close(parts)
}

// diff overlay
var diffOverlay image.Image

func main() {

	start := time.Now()

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

	chanPairs := make(chan Pair, x_iter*y_iter) // store Pairs for compare

	for y := 0; y < y_iter; y++ {
		for x := 0; x < x_iter; x++ {

			if x == x_iter && x_mod != 0 {
				x1 -= QUAD_SIZE + x_mod
			}

			if y == y_iter && y_mod != 0 {
				y1 -= QUAD_SIZE + y_mod
			}

			img_part1 := SubImage(imgOne, x0, y0, x1, y1)
			img_part2 := SubImage(imgTwo, x0, y0, x1, y1)

			buf_im1 := new(bytes.Buffer)
			buf_im2 := new(bytes.Buffer)
			err_im1 := png.Encode(buf_im1, img_part1)
			err_im2 := png.Encode(buf_im2, img_part2)

			if err_im1 != nil || err_im2 != nil {
				log.Fatalf("err_im1: = %v, err_im2 = %v", err_im1, err_im2)
			}

			if md5.Sum(buf_im1.Bytes()) == md5.Sum(buf_im2.Bytes()) {
				compared = append(compared, img_part1)
			} else {
				chanPairs <- Pair{
					Original: img_part1,
					Compared: img_part2,
				}
			}

			x0 += QUAD_SIZE
			x1 += QUAD_SIZE
		}

		x0 = 0
		x1 = QUAD_SIZE
		y0 += QUAD_SIZE
		y1 += QUAD_SIZE
	}

	parallelProcessDifferences(chanPairs, img1_width, img1_height)

	newImage := image.NewRGBA(image.Rect(0, 0, img1_width, img1_height))
	clr, _ := ToRGBA("FFFFFF")
	draw.Draw(newImage, newImage.Bounds(), image.NewUniform(clr), image.Point{}, draw.Src)

	for idx := range compared {
		draw.Draw(newImage, newImage.Bounds(), compared[idx], image.Point{0, 0}, draw.Over)
	}

	f, err := os.Create(RESULT_IMAGE_PATH)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()

	err = png.Encode(f, newImage)

	elapsed := time.Since(start)
	log.Printf("Diff took %s", elapsed)
}

// pixel-by-pixel image parts comparison
func diff(img1, img2 image.Image, wg *sync.WaitGroup) { //(image.Image, error) {

	defer wg.Done()

	bounds := img1.Bounds()

	w := bounds.Max.X
	h := bounds.Max.Y

	// TODO: change At to Pix and benchmark it
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {

			r1, g1, b1, _ := img1.At(x, y).RGBA()
			r2, g2, b2, _ := img2.At(x, y).RGBA()

			// NOTE: slow op
			// img1.(draw.Image).Set(x, y, color.GrayModel.Convert(img1.At(x, y)))

			if (r1 + g1 + b1) != (r2 + g2 + b2) {
				img1.(draw.Image).Set(x, y, color.RGBA{uint8(250), uint8(0), uint8(0), uint8(255)})
			}
		}
	}

	compared = append(compared, img1)
}
