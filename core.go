package main

import (
	"image"
	"image/color"
	"image/draw"
	"sync"
)

// pixel-by-pixel image parts comparison
func diff(img1, img2 image.Image, wg *sync.WaitGroup) { //(image.Image, error) {

	defer wg.Done()

	w := img1.Bounds().Max.X
	h := img1.Bounds().Max.Y

	// TODO: change At to Pix and benchmark it
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {

			r1, g1, b1, _ := img1.At(x, y).RGBA()
			r2, g2, b2, _ := img2.At(x, y).RGBA()

			if (r1 + g1 + b1) != (r2 + g2 + b2) {
				img1.(draw.Image).Set(x, y, combine(img1.At(x, y), color.RGBA{250, 0, 0, 220}))
			}
		}
	}
	compared = append(compared, img1)
}

// TODO: должны параллельно сразу все пары влететь, а не по одной
func parallelProcessDifferences(parts chan Pair) {

	var wg sync.WaitGroup

	// TODO: white unprocessed parts if
	// 	for x := 0; x < len(parts); x++ {

	lp := len(parts)
	for x := 0; x < lp; x++ {
		wg.Add(1)
		part := <-parts
		go diff(part.Original, part.Compared, &wg)
	}
	wg.Wait()

	close(parts)
}
