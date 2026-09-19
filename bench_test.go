package main

import (
	"bytes"
	"crypto/md5"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand"
	"strconv"
	"sync"
	"testing"
)

const (
	benchW = 1920
	benchH = 1080
)

// noise builds a deterministic photo-like image so that the PNG encoder in the
// legacy path has realistic work to do rather than compressing a flat colour.
func noise(w, h int, seed int64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(seed))
	_, _ = r.Read(img.Pix)
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	return img
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}

// benchCase runs Compare to completion, reporting bytes processed per second.
func benchCase(b *testing.B, base, current *image.RGBA) {
	b.ReportAllocs()
	b.SetBytes(int64(len(base.Pix)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := Compare(base, current, DefaultOptions()); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompareIdentical is the dominant case in CI: nothing changed.
func BenchmarkCompareIdentical(b *testing.B) {
	base := noise(benchW, benchH, 1)
	benchCase(b, base, cloneRGBA(base))
}

// BenchmarkCompareSparseDiff models a real regression: one small element moved.
func BenchmarkCompareSparseDiff(b *testing.B) {
	base := noise(benchW, benchH, 1)
	current := cloneRGBA(base)
	for y := 400; y < 500; y++ {
		for x := 600; x < 800; x++ {
			current.Set(x, y, color.RGBA{1, 2, 3, 255})
		}
	}
	benchCase(b, base, current)
}

// BenchmarkCompareAllDifferent is the worst case: every pixel differs.
func BenchmarkCompareAllDifferent(b *testing.B) {
	benchCase(b, noise(benchW, benchH, 1), noise(benchW, benchH, 2))
}

func BenchmarkCompareWorkers(b *testing.B) {
	base := noise(benchW, benchH, 1)
	current := noise(benchW, benchH, 2)

	for _, workers := range []int{1, 2, 4, 8} {
		b.Run("workers="+strconv.Itoa(workers), func(b *testing.B) {
			opts := DefaultOptions()
			opts.Workers = workers
			b.SetBytes(int64(len(base.Pix)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Compare(base, current, opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// --- legacy implementation, benchmarked for comparison only ----------------
//
// This reproduces the original algorithm: split both images into 100x100
// tiles, PNG-encode each tile to compare MD5 sums, then diff the surviving
// tiles pixel by pixel through the image.Image interface.

const legacyTile = 100

func legacySubImage(img image.Image, x1, y1, x2, y2 int) image.Image {
	return img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(x1, y1, x2, y2))
}

func legacyDiff(img1, img2 image.Image, wg *sync.WaitGroup, mu *sync.Mutex, compared *[]image.Image) {
	defer wg.Done()

	w := img1.Bounds().Max.X
	h := img1.Bounds().Max.Y

	for y := img1.Bounds().Min.Y; y < h; y++ {
		for x := img1.Bounds().Min.X; x < w; x++ {
			r1, g1, b1, _ := img1.At(x, y).RGBA()
			r2, g2, b2, _ := img2.At(x, y).RGBA()

			if (r1 + g1 + b1) != (r2 + g2 + b2) {
				img1.(draw.Image).Set(x, y, color.RGBA{250, 0, 0, 220})
			}
		}
	}

	mu.Lock()
	*compared = append(*compared, img1)
	mu.Unlock()
}

func legacyCompare(imgOne, imgTwo image.Image, w, h int) image.Image {
	xIter := w / legacyTile
	yIter := h / legacyTile
	if w%legacyTile > 0 {
		xIter++
	}
	if h%legacyTile > 0 {
		yIter++
	}

	var compared []image.Image
	var mu sync.Mutex
	var wg sync.WaitGroup

	for y := 0; y < yIter; y++ {
		for x := 0; x < xIter; x++ {
			x0, y0 := x*legacyTile, y*legacyTile
			x1, y1 := x0+legacyTile, y0+legacyTile

			part1 := legacySubImage(imgOne, x0, y0, x1, y1)
			part2 := legacySubImage(imgTwo, x0, y0, x1, y1)

			buf1 := new(bytes.Buffer)
			buf2 := new(bytes.Buffer)
			if err := png.Encode(buf1, part1); err != nil {
				panic(err)
			}
			if err := png.Encode(buf2, part2); err != nil {
				panic(err)
			}

			if md5.Sum(buf1.Bytes()) == md5.Sum(buf2.Bytes()) {
				compared = append(compared, part1)
				continue
			}

			wg.Add(1)
			go legacyDiff(part1, part2, &wg, &mu, &compared)
		}
	}
	wg.Wait()

	result := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(result, result.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for idx := range compared {
		draw.Draw(result, result.Bounds(), compared[idx], image.Point{}, draw.Src)
	}
	return result
}

func benchLegacy(b *testing.B, base, current *image.RGBA) {
	b.ReportAllocs()
	b.SetBytes(int64(len(base.Pix)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// The legacy path writes into its inputs, so restore them each run.
		in1, in2 := cloneRGBA(base), cloneRGBA(current)
		b.StartTimer()

		legacyCompare(in1, in2, benchW, benchH)
	}
}

func BenchmarkLegacyIdentical(b *testing.B) {
	base := noise(benchW, benchH, 1)
	benchLegacy(b, base, cloneRGBA(base))
}

func BenchmarkLegacyAllDifferent(b *testing.B) {
	benchLegacy(b, noise(benchW, benchH, 1), noise(benchW, benchH, 2))
}

// uiLike builds a flat-colour, screenshot-shaped image: large uniform regions
// that PNG compresses well. This is the favourable case for the legacy path,
// where hashing tiles is cheapest.
func uiLike(w, h int, seed int64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{250, 250, 252, 255}), image.Point{}, draw.Src)

	r := rand.New(rand.NewSource(seed))
	for i := 0; i < 60; i++ {
		x, y := r.Intn(w-200), r.Intn(h-120)
		block := image.Rect(x, y, x+r.Intn(200)+20, y+r.Intn(120)+16)
		shade := color.RGBA{uint8(r.Intn(200)), uint8(r.Intn(200)), uint8(r.Intn(220)), 255}
		draw.Draw(img, block, image.NewUniform(shade), image.Point{}, draw.Src)
	}
	return img
}

func BenchmarkCompareUILike(b *testing.B) {
	base := uiLike(benchW, benchH, 7)
	benchCase(b, base, cloneRGBA(base))
}

func BenchmarkLegacyUILike(b *testing.B) {
	base := uiLike(benchW, benchH, 7)
	benchLegacy(b, base, cloneRGBA(base))
}

// Antialiasing detection runs only on pixels that already differ, so its cost
// should track the size of the change, not the size of the image.
func BenchmarkCompareAntialiasIdentical(b *testing.B) {
	base := uiLike(benchW, benchH, 7)
	current := cloneRGBA(base)

	opts := DefaultOptions()
	opts.IgnoreAntialiasing = true

	b.SetBytes(int64(len(base.Pix)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Compare(base, current, opts); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompareAntialiasSparse(b *testing.B) {
	base := uiLike(benchW, benchH, 7)
	current := cloneRGBA(base)
	for y := 400; y < 500; y++ {
		for x := 600; x < 800; x++ {
			current.Set(x, y, color.RGBA{1, 2, 3, 255})
		}
	}

	opts := DefaultOptions()
	opts.IgnoreAntialiasing = true

	b.SetBytes(int64(len(base.Pix)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Compare(base, current, opts); err != nil {
			b.Fatal(err)
		}
	}
}

// Worst case: every pixel differs, so every pixel is also checked for
// antialiasing.
func BenchmarkCompareAntialiasAllDifferent(b *testing.B) {
	base := noise(benchW, benchH, 1)
	current := noise(benchW, benchH, 2)

	opts := DefaultOptions()
	opts.IgnoreAntialiasing = true

	b.SetBytes(int64(len(base.Pix)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Compare(base, current, opts); err != nil {
			b.Fatal(err)
		}
	}
}
