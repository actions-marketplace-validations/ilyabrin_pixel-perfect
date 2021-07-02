package main

import (
	"fmt"
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
   4.    - save result into new image
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

var imageParts1 []image.Image
var imageParts2 []image.Image

func main() {

	numArgs := len(os.Args[1:])
	if numArgs != 2 {
		os.Exit(1)
	}

	arg1 := os.Args[1]
	arg2 := os.Args[2]

	fmt.Println(arg1, arg2)

	if _, err := os.Stat(arg1); os.IsNotExist(err) {
		log.Fatalf("File %s is not exists", arg1)
	}

	if _, err := os.Stat(arg2); os.IsNotExist(err) {
		log.Fatalf("File %s is not exists", arg2)
	}

	// [ok] - открыть файлы если они есть
	// [ok] - узнать ширину и высоту
	// [ok] - есть они разные - выйти
	// [ok] - делим обе картинки на квадраты 100x100
	// [--] - каждый квадрат нумеруем и помещаем в мапу или канал
	// [--] - должно выглядеть как map[int]React (мапа будет последовательно читать)
	// [--] - затем сравниваем покусочно map1[int]image100x100 == map2[int]100x100
	// [--] - если нет различий - идем дальше и проверяем следующие два квадрата
	// [--] - если различия есть - записываем в результирующую мапу новый квадрат с красными пикселями
	// [--] - если есть различия - создаем issue с лейблом UI collision
	// [--] - если ошибок нет - кайфуем

	// os.Exit(1)

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

	fmt.Printf("\nY: %d \n X: %d \n Mod_X: %d \n Mod_Y: %d \n", y_iter, x_iter, x_mod, y_mod)

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
			// todo: send to chan with parts

			if x == x_iter && x_mod != 0 {
				x1 -= QUAD_SIZE + x_mod
			}

			if y == y_iter && y_mod != 0 {
				y1 -= QUAD_SIZE + y_mod
			}

			fmt.Println(x0, y0, x1, y1)

			// (O_o) rewrite ASAP!
			img_part1 := imgOne.(interface {
				SubImage(r image.Rectangle) image.Image
			}).SubImage(image.Rect(x0, y0, x1, y1))
			imageParts1 = append(imageParts1, img_part1)

			img_part2 := imgTwo.(interface {
				SubImage(r image.Rectangle) image.Image
			}).SubImage(image.Rect(x0, y0, x1, y1))
			imageParts2 = append(imageParts2, img_part2)

			// f, err := os.Create("./parts/part_" + fmt.Sprint(x) + fmt.Sprint(y) + ".png")
			// if err != nil {
			// 	log.Println(err)
			// }
			// defer f.Close()

			// err = png.Encode(f, img_part1)
			// if err != nil {
			// 	log.Println(err)
			// }

			x0 += QUAD_SIZE
			x1 += QUAD_SIZE
		}

		x0 = 0
		x1 = QUAD_SIZE
		y0 += QUAD_SIZE
		y1 += QUAD_SIZE
	}

	//////
	log.Println("KEK", len(imageParts1))

	newImage := image.NewRGBA(image.Rect(0, 0, img1_width, img1_height))

	clr, _ := ToRGBA("FFFFFF")

	draw.Draw(newImage, newImage.Bounds(), image.NewUniform(clr), image.Point{}, draw.Src)

	for idx := range imageParts1 {
		// println(idx)b
		// println(imageParts1[idx] == imageParts2[idx])

		// f, err := os.Create("./parts/part_" + fmt.Sprint(idx) + ".png")
		// if err != nil {
		// 	log.Println(err)
		// }
		// defer f.Close()

		draw.Draw(newImage, newImage.Bounds(), imageParts1[idx], image.Point{0, 0}, draw.Src)

		// err = png.Encode(f, imageParts1[idx])
		// if err != nil {
		// 	log.Println(err)
		// }
	}

	f, err := os.Create("./parts/result.png")
	if err != nil {
		log.Println(err)
	}
	defer f.Close()

	err = png.Encode(f, newImage)
	os.Exit(1)

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
		log.Fatalf("file not found: %s", path_img1)
	}
	defer img1.Close()

	img2, err := os.Open(path_img2)

	if err != nil {
		log.Fatalf("file not found: %s", path_img2)
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

	log.Println("Image 1 width : ", img1w)
	log.Println("Image 1 height : ", img1h)

	log.Println("Image 2 width : ", img2w)
	log.Println("Image 2 height : ", img2h)

	if img1h != img2h || img1w != img2w {
		return false
	}
	return true
}
