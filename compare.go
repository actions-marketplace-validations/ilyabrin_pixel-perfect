package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"runtime"
	"sync"
)

// ErrSizeMismatch is returned when the two images do not share identical bounds.
var ErrSizeMismatch = errors.New("images must have the same dimensions")

// Options controls how two images are compared and how the diff is rendered.
type Options struct {
	// Threshold is the maximum per-channel delta (0-255) still considered equal.
	// Zero means an exact match is required.
	Threshold uint8

	// Highlight is the colour painted over differing pixels. Its alpha channel
	// controls how much of the original pixel remains visible.
	Highlight color.RGBA

	// Workers is the number of goroutines used. Zero means runtime.NumCPU().
	Workers int
}

// DefaultOptions renders differences as semi-transparent red and requires an
// exact pixel match.
func DefaultOptions() Options {
	return Options{
		Threshold: 0,
		Highlight: color.RGBA{R: 250, G: 0, B: 0, A: 200},
		Workers:   0,
	}
}

// Result holds the outcome of a comparison.
type Result struct {
	// Image is a copy of the baseline with differing pixels highlighted.
	Image *image.RGBA

	// DiffPixels is how many pixels exceeded the threshold.
	DiffPixels int

	// TotalPixels is the pixel count of either image.
	TotalPixels int
}

// Ratio is the fraction of pixels that differ, in the range [0,1].
func (r Result) Ratio() float64 {
	if r.TotalPixels == 0 {
		return 0
	}
	return float64(r.DiffPixels) / float64(r.TotalPixels)
}

// Equal reports whether the images matched within the configured threshold.
func (r Result) Equal() bool { return r.DiffPixels == 0 }

// Compare diffs two images of identical size. It returns a copy of base with
// every differing pixel blended with opts.Highlight.
//
// Work is split into horizontal bands, one per worker. Bands never overlap in
// the output buffer, so no synchronisation is needed on the pixel data itself.
func Compare(base, current *image.RGBA, opts Options) (Result, error) {
	if base.Bounds() != current.Bounds() {
		return Result{}, ErrSizeMismatch
	}

	b := base.Bounds()
	w, h := b.Dx(), b.Dy()

	out := image.NewRGBA(image.Rect(0, 0, w, h))
	// A single copy is far cheaper than per-pixel writes; workers then only
	// touch the pixels that actually differ.
	copy(out.Pix, base.Pix)

	res := Result{Image: out, TotalPixels: w * h}
	if w == 0 || h == 0 {
		return res, nil
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > h {
		workers = h
	}

	counts := make([]int, workers)
	rows := (h + workers - 1) / workers

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		y0 := i * rows
		if y0 >= h {
			break
		}
		y1 := y0 + rows
		if y1 > h {
			y1 = h
		}

		wg.Add(1)
		go func(i, y0, y1 int) {
			defer wg.Done()
			counts[i] = diffBand(base, current, out, y0, y1, opts)
		}(i, y0, y1)
	}
	wg.Wait()

	for _, c := range counts {
		res.DiffPixels += c
	}
	return res, nil
}

// diffBand compares rows [y0,y1) and paints differences into out.
func diffBand(base, current, out *image.RGBA, y0, y1 int, opts Options) int {
	w := base.Bounds().Dx()
	rowLen := w * 4
	thr := int(opts.Threshold)
	hl := opts.Highlight

	var count int
	for y := y0; y < y1; y++ {
		i := base.PixOffset(base.Bounds().Min.X, base.Bounds().Min.Y+y)
		j := current.PixOffset(current.Bounds().Min.X, current.Bounds().Min.Y+y)

		rowA := base.Pix[i : i+rowLen]
		rowB := current.Pix[j : j+rowLen]

		// Fast path: identical rows are the common case in visual regression
		// runs, and bytes.Equal uses SIMD-backed assembly.
		if bytes.Equal(rowA, rowB) {
			continue
		}

		k := out.PixOffset(0, y)
		rowOut := out.Pix[k : k+rowLen]

		for x := 0; x < rowLen; x += 4 {
			if !pixelDiffers(rowA[x:x+4:x+4], rowB[x:x+4:x+4], thr) {
				continue
			}
			count++
			blend(rowOut[x:x+4:x+4], hl)
		}
	}
	return count
}

// pixelDiffers reports whether any channel differs by more than thr.
func pixelDiffers(a, b []uint8, thr int) bool {
	if thr == 0 {
		// Compared as one 32-bit word by the compiler on most architectures.
		return a[0] != b[0] || a[1] != b[1] || a[2] != b[2] || a[3] != b[3]
	}
	for c := 0; c < 4; c++ {
		d := int(a[c]) - int(b[c])
		if d < 0 {
			d = -d
		}
		if d > thr {
			return true
		}
	}
	return false
}

// blend paints hl over dst using hl's alpha as the mix factor.
func blend(dst []uint8, hl color.RGBA) {
	a := uint32(hl.A)
	inv := 255 - a
	dst[0] = uint8((uint32(hl.R)*a + uint32(dst[0])*inv) / 255)
	dst[1] = uint8((uint32(hl.G)*a + uint32(dst[1])*inv) / 255)
	dst[2] = uint8((uint32(hl.B)*a + uint32(dst[2])*inv) / 255)
	dst[3] = 255
}
