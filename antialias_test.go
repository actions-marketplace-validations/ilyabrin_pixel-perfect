package main

import (
	"image"
	"image/color"
	"testing"
)

// diagonal draws a dark shape on white with a smoothed edge. shift moves the
// edge by a sub-pixel amount, which is exactly what a re-render does when font
// hinting or scaling lands slightly differently.
func diagonal(w, h int, shift float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Distance from the edge line, in pixels.
			d := float64(x) - (float64(y)*0.7 + 10 + shift)

			var v float64
			switch {
			case d <= -1:
				v = 0 // inside the shape
			case d >= 1:
				v = 255 // outside
			default:
				v = (d + 1) / 2 * 255 // the antialiased band
			}

			c := uint8(v)
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c, c, c, 255
		}
	}
	return img
}

// The point of the feature: a re-rendered edge must stop failing the run.
func TestIgnoreAntialiasingAbsorbsEdgeJitter(t *testing.T) {
	a := diagonal(120, 80, 0)
	b := diagonal(120, 80, 0.3)

	strict := compareOrFatal(t, a, b, DefaultOptions())
	if strict.DiffPixels == 0 {
		t.Fatal("the fixture does not differ at all; the test proves nothing")
	}

	opts := DefaultOptions()
	opts.IgnoreAntialiasing = true
	relaxed := compareOrFatal(t, a, b, opts)

	if relaxed.DiffPixels >= strict.DiffPixels {
		t.Errorf("antialiasing detection changed nothing: %d differing pixels with it, %d without",
			relaxed.DiffPixels, strict.DiffPixels)
	}
	if relaxed.AntialiasedPixels == 0 {
		t.Error("AntialiasedPixels = 0, want the skipped pixels to be counted")
	}
	if relaxed.DiffPixels+relaxed.AntialiasedPixels != strict.DiffPixels {
		t.Errorf("pixels went missing: %d + %d != %d",
			relaxed.DiffPixels, relaxed.AntialiasedPixels, strict.DiffPixels)
	}
}

// The risk with this feature is hiding real regressions. It must not.
func TestIgnoreAntialiasingStillCatchesRealChanges(t *testing.T) {
	opts := DefaultOptions()
	opts.IgnoreAntialiasing = true

	t.Run("a moved block", func(t *testing.T) {
		a := solid(120, 80, white)
		b := solid(120, 80, white)
		drawBlock(a, 20, 20, 60, 40, black)
		drawBlock(b, 28, 20, 68, 40, black) // shifted 8px

		res := compareOrFatal(t, a, b, opts)
		if res.DiffPixels == 0 {
			t.Error("an 8px shift was absorbed as antialiasing")
		}
	})

	t.Run("a recoloured region", func(t *testing.T) {
		a := solid(120, 80, white)
		b := solid(120, 80, white)
		drawBlock(a, 20, 20, 60, 40, color.RGBA{40, 90, 200, 255})
		drawBlock(b, 20, 20, 60, 40, color.RGBA{200, 40, 40, 255})

		res := compareOrFatal(t, a, b, opts)
		if res.DiffPixels < 600 {
			t.Errorf("a recoloured 40x20 block reported only %d differing pixels", res.DiffPixels)
		}
	})

	t.Run("a wholly different image", func(t *testing.T) {
		a := solid(64, 64, white)
		b := solid(64, 64, black)

		res := compareOrFatal(t, a, b, opts)
		if res.DiffPixels != 64*64 {
			t.Errorf("DiffPixels = %d, want every pixel to differ", res.DiffPixels)
		}
	})
}

// A flat region is not an edge, so nothing there may be written off as
// antialiasing.
func TestIgnoreAntialiasingLeavesFlatAreasAlone(t *testing.T) {
	a := solid(64, 64, color.RGBA{128, 128, 128, 255})
	b := solid(64, 64, color.RGBA{128, 128, 128, 255})
	drawBlock(b, 10, 10, 30, 30, color.RGBA{138, 138, 138, 255})

	opts := DefaultOptions()
	opts.IgnoreAntialiasing = true
	res := compareOrFatal(t, a, b, opts)

	if res.DiffPixels != 400 {
		t.Errorf("DiffPixels = %d, want 400: a flat patch is not antialiasing", res.DiffPixels)
	}
	if res.AntialiasedPixels != 0 {
		t.Errorf("AntialiasedPixels = %d, want 0", res.AntialiasedPixels)
	}
}

func TestIgnoreAntialiasingOffByDefault(t *testing.T) {
	if DefaultOptions().IgnoreAntialiasing {
		t.Error("IgnoreAntialiasing should be opt-in")
	}

	a := diagonal(60, 40, 0)
	b := diagonal(60, 40, 0.3)

	res := compareOrFatal(t, a, b, DefaultOptions())
	if res.AntialiasedPixels != 0 {
		t.Errorf("AntialiasedPixels = %d, want 0 when the option is off", res.AntialiasedPixels)
	}
}

// Identical images must stay identical, and cost nothing extra.
func TestIgnoreAntialiasingOnIdenticalImages(t *testing.T) {
	a := diagonal(60, 40, 0)
	b := diagonal(60, 40, 0)

	opts := DefaultOptions()
	opts.IgnoreAntialiasing = true
	res := compareOrFatal(t, a, b, opts)

	if !res.Equal() || res.AntialiasedPixels != 0 {
		t.Errorf("identical images reported %d differing and %d antialiased pixels",
			res.DiffPixels, res.AntialiasedPixels)
	}
}

func TestLuminanceCompositesTranslucentPixelsOnWhite(t *testing.T) {
	opaqueWhite := luminance(255, 255, 255, 255)
	transparent := luminance(0, 0, 0, 0)

	if transparent != opaqueWhite {
		t.Errorf("a fully transparent pixel reads as %v, want it composited onto white (%v)",
			transparent, opaqueWhite)
	}

	if luminance(0, 0, 0, 255) >= luminance(255, 255, 255, 255) {
		t.Error("black should be darker than white")
	}
}

func TestHasManySiblings(t *testing.T) {
	img := solid(10, 10, white)
	if !hasManySiblings(img, 5, 5) {
		t.Error("a pixel inside a flat area should have many siblings")
	}

	// A lone pixel surrounded entirely by another colour has none.
	lone := solid(10, 10, white)
	lone.SetRGBA(5, 5, black)
	if hasManySiblings(lone, 5, 5) {
		t.Error("an isolated pixel should not count as part of a flat run")
	}

	// A border pixel counts its missing neighbours as one sibling, so two
	// real ones are enough.
	if !hasManySiblings(img, 0, 0) {
		t.Error("a corner pixel of a flat area should have many siblings")
	}
}

// drawBlock fills a rectangle, used to build fixtures.
func drawBlock(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}
