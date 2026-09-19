package main

import (
	"image"
	"strings"
	"testing"
)

func TestParseRegion(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    image.Rectangle
		wantErr string
	}{
		{in: "0,0,320,64", want: image.Rect(0, 0, 320, 64)},
		{in: "10, 20, 30, 40", want: image.Rect(10, 20, 40, 60)},
		{in: "1,2,3", wantErr: "x,y,width,height"},
		{in: "1,2,3,4,5", wantErr: "x,y,width,height"},
		{in: "a,2,3,4", wantErr: "not a number"},
		{in: "0,0,0,10", wantErr: "must be positive"},
		{in: "0,0,10,-1", wantErr: "must be positive"},
		{in: "-1,0,10,10", wantErr: "must not be negative"},
	} {
		got, err := ParseRegion(tc.in)

		if tc.wantErr != "" {
			if err == nil {
				t.Errorf("ParseRegion(%q): expected an error, got %v", tc.in, got)
			} else if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("ParseRegion(%q): error = %v, want it to mention %q", tc.in, err, tc.wantErr)
			}
			continue
		}

		if err != nil {
			t.Errorf("ParseRegion(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseRegion(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// One flag occurrence may carry several regions, which is how the action
// passes a list through a single input.
func TestRegionsSetAcceptsSemicolonList(t *testing.T) {
	var r Regions
	if err := r.Set("0,0,10,10; 20,20,5,5"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := r.Set("100,100,1,1"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	want := []image.Rectangle{
		image.Rect(0, 0, 10, 10),
		image.Rect(20, 20, 25, 25),
		image.Rect(100, 100, 101, 101),
	}
	if len(r) != len(want) {
		t.Fatalf("got %d regions (%v), want %d", len(r), r, len(want))
	}
	for i := range want {
		if r[i] != want[i] {
			t.Errorf("region %d = %v, want %v", i, r[i], want[i])
		}
	}

	if got := r.String(); !strings.Contains(got, "0,0,10,10") {
		t.Errorf("String() = %q, want it to round-trip the flag syntax", got)
	}
}

func TestRegionsIgnoredPixelsCountsOverlapOnce(t *testing.T) {
	r := Regions{
		image.Rect(0, 0, 10, 10),
		image.Rect(5, 5, 15, 15), // overlaps the first by 5x5
	}

	// 100 + 100 - 25 overlapping
	if got := r.IgnoredPixels(100, 100); got != 175 {
		t.Errorf("IgnoredPixels = %d, want 175 (overlap counted once)", got)
	}

	// Regions are clipped to the image.
	clipped := Regions{image.Rect(90, 90, 200, 200)}
	if got := clipped.IgnoredPixels(100, 100); got != 100 {
		t.Errorf("IgnoredPixels = %d, want 100 (clipped to the image)", got)
	}

	if got := (Regions{}).IgnoredPixels(100, 100); got != 0 {
		t.Errorf("IgnoredPixels = %d, want 0 with no regions", got)
	}
}

// The feature itself: a region that changes every run must not fail the build.
func TestCompareIgnoresRegion(t *testing.T) {
	a := solid(100, 100, white)
	b := solid(100, 100, white)
	drawBlock(b, 10, 10, 30, 30, black) // 400 pixels of "dynamic content"

	strict := compareOrFatal(t, a, b, DefaultOptions())
	if strict.DiffPixels != 400 {
		t.Fatalf("DiffPixels = %d, want 400 before ignoring", strict.DiffPixels)
	}

	opts := DefaultOptions()
	opts.Ignore = Regions{image.Rect(10, 10, 30, 30)}
	ignored := compareOrFatal(t, a, b, opts)

	if !ignored.Equal() {
		t.Errorf("DiffPixels = %d, want 0 once the region is ignored", ignored.DiffPixels)
	}
	if ignored.IgnoredPixels != 400 {
		t.Errorf("IgnoredPixels = %d, want 400", ignored.IgnoredPixels)
	}
	// The ignored area must leave the denominator too, or the ratio is wrong.
	if ignored.TotalPixels != 100*100-400 {
		t.Errorf("TotalPixels = %d, want %d", ignored.TotalPixels, 100*100-400)
	}
}

// Changes outside the region must still be caught.
func TestCompareIgnoreDoesNotMaskNeighbouringChanges(t *testing.T) {
	a := solid(100, 100, white)
	b := solid(100, 100, white)
	drawBlock(b, 10, 10, 30, 30, black) // inside the region
	drawBlock(b, 50, 50, 60, 60, black) // outside it

	opts := DefaultOptions()
	opts.Ignore = Regions{image.Rect(10, 10, 30, 30)}
	res := compareOrFatal(t, a, b, opts)

	if res.DiffPixels != 100 {
		t.Errorf("DiffPixels = %d, want 100: only the change outside the region counts", res.DiffPixels)
	}
	if res.Image.RGBAAt(55, 55) == white {
		t.Error("the change outside the region was not highlighted")
	}
	if res.Image.RGBAAt(15, 15) != white {
		t.Error("a pixel inside the ignored region was highlighted")
	}
}

func TestCompareMultipleIgnoredRegions(t *testing.T) {
	a := solid(100, 100, white)
	b := solid(100, 100, white)
	drawBlock(b, 0, 0, 20, 10, black)     // a header timestamp
	drawBlock(b, 80, 90, 100, 100, black) // a footer counter
	drawBlock(b, 40, 40, 50, 50, black)   // a real change

	opts := DefaultOptions()
	opts.Ignore = Regions{
		image.Rect(0, 0, 20, 10),
		image.Rect(80, 90, 100, 100),
	}
	res := compareOrFatal(t, a, b, opts)

	if res.DiffPixels != 100 {
		t.Errorf("DiffPixels = %d, want 100 from the real change alone", res.DiffPixels)
	}
}

// A region larger than the image is clipped rather than rejected.
func TestCompareIgnoreRegionLargerThanImage(t *testing.T) {
	a := solid(50, 50, white)
	b := solid(50, 50, black)

	opts := DefaultOptions()
	opts.Ignore = Regions{image.Rect(0, 0, 500, 500)}
	res := compareOrFatal(t, a, b, opts)

	if res.DiffPixels != 0 {
		t.Errorf("DiffPixels = %d, want 0: everything is ignored", res.DiffPixels)
	}
	if res.TotalPixels != 0 {
		t.Errorf("TotalPixels = %d, want 0", res.TotalPixels)
	}
	if res.Ratio() != 0 {
		t.Errorf("Ratio() = %v, want 0 rather than a division by zero", res.Ratio())
	}
}

func TestCompareIgnoreWorksWithEveryWorkerCount(t *testing.T) {
	a := solid(64, 37, white)
	b := solid(64, 37, white)
	drawBlock(b, 5, 5, 15, 15, black)   // ignored
	drawBlock(b, 40, 25, 45, 30, black) // counted

	for _, workers := range []int{1, 2, 3, 8, 37} {
		opts := DefaultOptions()
		opts.Ignore = Regions{image.Rect(5, 5, 15, 15)}
		opts.Workers = workers

		res := compareOrFatal(t, a, b, opts)
		if res.DiffPixels != 25 {
			t.Errorf("workers=%d: DiffPixels = %d, want 25", workers, res.DiffPixels)
		}
	}
}
