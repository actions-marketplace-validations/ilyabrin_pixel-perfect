package main

import (
	"errors"
	"image"
	"image/color"
	"testing"
)

// solid builds a w*h image filled with c.
func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i+0] = c.R
		img.Pix[i+1] = c.G
		img.Pix[i+2] = c.B
		img.Pix[i+3] = c.A
	}
	return img
}

var (
	white = color.RGBA{255, 255, 255, 255}
	black = color.RGBA{0, 0, 0, 255}
)

func compareOrFatal(t *testing.T, a, b *image.RGBA, opts Options) Result {
	t.Helper()
	res, err := Compare(a, b, opts)
	if err != nil {
		t.Fatalf("Compare returned unexpected error: %v", err)
	}
	return res
}

func TestCompareIdenticalImages(t *testing.T) {
	a := solid(64, 48, white)
	b := solid(64, 48, white)

	res := compareOrFatal(t, a, b, DefaultOptions())

	if !res.Equal() {
		t.Errorf("identical images reported %d differing pixels", res.DiffPixels)
	}
	if res.Ratio() != 0 {
		t.Errorf("Ratio() = %v, want 0", res.Ratio())
	}
	if res.TotalPixels != 64*48 {
		t.Errorf("TotalPixels = %d, want %d", res.TotalPixels, 64*48)
	}
}

func TestCompareSinglePixelIsLocatedExactly(t *testing.T) {
	a := solid(64, 48, white)
	b := solid(64, 48, white)
	b.Set(10, 20, black)

	res := compareOrFatal(t, a, b, DefaultOptions())

	if res.DiffPixels != 1 {
		t.Fatalf("DiffPixels = %d, want 1", res.DiffPixels)
	}

	// The highlight must land on the pixel that actually changed. The previous
	// implementation drew tiles at the wrong offset, so assert the position.
	if got := res.Image.RGBAAt(10, 20); got == white {
		t.Error("changed pixel at (10,20) was not highlighted")
	}
	if got := res.Image.RGBAAt(11, 20); got != white {
		t.Errorf("unchanged pixel at (11,20) was modified: %v", got)
	}
	if got := res.Image.RGBAAt(0, 0); got != white {
		t.Errorf("unchanged pixel at (0,0) was modified: %v", got)
	}
}

// Summing channels treats (10,0,0) and (0,10,0) as equal. They are not.
func TestCompareDetectsChannelSwap(t *testing.T) {
	a := solid(8, 8, color.RGBA{10, 0, 0, 255})
	b := solid(8, 8, color.RGBA{0, 10, 0, 255})

	res := compareOrFatal(t, a, b, DefaultOptions())

	if res.DiffPixels != 64 {
		t.Errorf("DiffPixels = %d, want 64: channel swap must not cancel out", res.DiffPixels)
	}
}

func TestCompareDetectsAlphaOnlyChange(t *testing.T) {
	a := solid(8, 8, color.RGBA{255, 255, 255, 255})
	b := solid(8, 8, color.RGBA{255, 255, 255, 128})

	res := compareOrFatal(t, a, b, DefaultOptions())

	if res.DiffPixels != 64 {
		t.Errorf("DiffPixels = %d, want 64: alpha changes must be detected", res.DiffPixels)
	}
}

// Sizes that are not a multiple of any internal band size must still be fully
// scanned, including the final partial band.
func TestCompareNonDivisibleDimensions(t *testing.T) {
	for _, size := range []struct{ w, h int }{
		{150, 150}, {1, 1}, {1, 401}, {401, 1}, {3, 7}, {99, 101},
	} {
		a := solid(size.w, size.h, white)
		b := solid(size.w, size.h, white)
		b.Set(size.w-1, size.h-1, black) // bottom-right: the last pixel scanned

		res := compareOrFatal(t, a, b, DefaultOptions())

		if res.DiffPixels != 1 {
			t.Errorf("%dx%d: DiffPixels = %d, want 1 (last pixel must be scanned)",
				size.w, size.h, res.DiffPixels)
		}
		if res.TotalPixels != size.w*size.h {
			t.Errorf("%dx%d: TotalPixels = %d, want %d",
				size.w, size.h, res.TotalPixels, size.w*size.h)
		}
	}
}

// Every worker count must produce the same answer, including counts that do not
// divide the height evenly and counts larger than the height.
func TestCompareWorkerCountDoesNotChangeResult(t *testing.T) {
	a := solid(37, 23, white)
	b := solid(37, 23, white)
	for _, p := range []image.Point{{0, 0}, {36, 22}, {18, 11}, {5, 17}} {
		b.Set(p.X, p.Y, black)
	}

	for _, workers := range []int{1, 2, 3, 7, 8, 23, 64} {
		opts := DefaultOptions()
		opts.Workers = workers

		res := compareOrFatal(t, a, b, opts)

		if res.DiffPixels != 4 {
			t.Errorf("workers=%d: DiffPixels = %d, want 4", workers, res.DiffPixels)
		}
	}
}

func TestCompareThreshold(t *testing.T) {
	a := solid(16, 16, color.RGBA{100, 100, 100, 255})
	b := solid(16, 16, color.RGBA{105, 100, 100, 255})

	withinThreshold := DefaultOptions()
	withinThreshold.Threshold = 5
	if res := compareOrFatal(t, a, b, withinThreshold); !res.Equal() {
		t.Errorf("delta 5 with threshold 5: got %d differing pixels, want 0", res.DiffPixels)
	}

	belowThreshold := DefaultOptions()
	belowThreshold.Threshold = 4
	if res := compareOrFatal(t, a, b, belowThreshold); res.DiffPixels != 256 {
		t.Errorf("delta 5 with threshold 4: got %d differing pixels, want 256", res.DiffPixels)
	}
}

func TestCompareSizeMismatch(t *testing.T) {
	a := solid(10, 10, white)
	b := solid(10, 11, white)

	_, err := Compare(a, b, DefaultOptions())

	if !errors.Is(err, ErrSizeMismatch) {
		t.Errorf("err = %v, want ErrSizeMismatch", err)
	}
}

// Compare must not modify its inputs; only the returned image is written to.
func TestCompareDoesNotMutateInputs(t *testing.T) {
	a := solid(32, 32, white)
	b := solid(32, 32, white)
	b.Set(4, 4, black)

	beforeA := append([]uint8(nil), a.Pix...)
	beforeB := append([]uint8(nil), b.Pix...)

	compareOrFatal(t, a, b, DefaultOptions())

	for i := range beforeA {
		if a.Pix[i] != beforeA[i] {
			t.Fatalf("baseline image was mutated at byte %d", i)
		}
	}
	for i := range beforeB {
		if b.Pix[i] != beforeB[i] {
			t.Fatalf("current image was mutated at byte %d", i)
		}
	}
}

func TestCompareEmptyImage(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 0, 0))
	b := image.NewRGBA(image.Rect(0, 0, 0, 0))

	res := compareOrFatal(t, a, b, DefaultOptions())

	if res.Ratio() != 0 || res.DiffPixels != 0 {
		t.Errorf("empty image: got ratio %v, %d pixels; want 0, 0", res.Ratio(), res.DiffPixels)
	}
}

func TestParseHexColor(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    color.RGBA
		wantErr bool
	}{
		{in: "FF0000", want: color.RGBA{255, 0, 0, 200}},
		{in: "#00FF00", want: color.RGBA{0, 255, 0, 200}},
		{in: "0000ff", want: color.RGBA{0, 0, 255, 200}},
		{in: "FFF", wantErr: true},
		{in: "GGGGGG", wantErr: true},
		{in: "", wantErr: true},
	} {
		got, err := ParseHexColor(tc.in, 200)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseHexColor(%q): expected an error, got %v", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseHexColor(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseHexColor(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
