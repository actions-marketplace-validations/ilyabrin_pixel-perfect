package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withMaxPixels swaps the ceiling for the duration of a test.
func withMaxPixels(t *testing.T, n int) {
	t.Helper()
	previous := maxPixels
	maxPixels = n
	t.Cleanup(func() { maxPixels = previous })
}

func TestLoadRGBARefusesOversizedImages(t *testing.T) {
	path := writePNG(t, "big.png", 64, 64, white, nil)

	withMaxPixels(t, 1000) // 64x64 is 4096 pixels

	_, err := LoadRGBA(path)
	if err == nil {
		t.Fatal("expected an oversized image to be refused")
	}
	for _, want := range []string{"64x64", "limit", "max-pixels"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to mention %q", err, want)
		}
	}
}

func TestLoadRGBAAllowsImagesWithinTheLimit(t *testing.T) {
	path := writePNG(t, "ok.png", 64, 64, white, nil)

	withMaxPixels(t, 4096) // exactly the pixel count

	if _, err := LoadRGBA(path); err != nil {
		t.Errorf("an image exactly at the limit was refused: %v", err)
	}
}

func TestLoadRGBALimitCanBeDisabled(t *testing.T) {
	path := writePNG(t, "any.png", 64, 64, white, nil)

	withMaxPixels(t, 0)

	if _, err := LoadRGBA(path); err != nil {
		t.Errorf("the check should be off with maxPixels 0: %v", err)
	}
}

// The ceiling must be applied from the header, before any pixels are
// allocated. A truncated file proves the dimensions were read on their own.
func TestLoadRGBAChecksSizeBeforeDecoding(t *testing.T) {
	full, err := os.ReadFile(writePNG(t, "full.png", 64, 64, white, nil))
	if err != nil {
		t.Fatal(err)
	}

	// Keep the header, drop the pixel data. DecodeConfig still succeeds,
	// Decode cannot.
	truncated := filepath.Join(t.TempDir(), "truncated.png")
	if err := os.WriteFile(truncated, full[:60], 0o644); err != nil {
		t.Fatal(err)
	}

	withMaxPixels(t, 1000)

	_, err = LoadRGBA(truncated)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "limit") {
		t.Errorf("error = %q, want the size limit to be reported rather than a decode failure", err)
	}
}

// The multiplication must not overflow on absurd dimensions.
func TestCheckSizeHandlesHugeDimensions(t *testing.T) {
	withMaxPixels(t, DefaultMaxPixels)

	if err := checkSize("huge.png", 1<<30, 1<<30); err == nil {
		t.Error("expected an error rather than an overflowed product")
	}
}

func TestCheckSizeRejectsEmptyDimensions(t *testing.T) {
	for _, d := range [][2]int{{0, 10}, {10, 0}, {-1, 10}} {
		if err := checkSize("x.png", d[0], d[1]); err == nil {
			t.Errorf("%dx%d: expected an error", d[0], d[1])
		}
	}
}

func TestRunMaxPixelsFlag(t *testing.T) {
	// run() assigns the global ceiling from the flag, so restore it or the
	// value leaks into whatever test runs next.
	withMaxPixels(t, maxPixels)

	a := writePNG(t, "a.png", 64, 64, white, nil)

	code, _, stderr := runCLI(t, "-base", a, "-current", a, "-max-pixels", "1000")
	if code != exitBadUsage {
		t.Errorf("exit code = %d, want %d", code, exitBadUsage)
	}
	if !strings.Contains(stderr, "megapixel limit") {
		t.Errorf("stderr = %q, want it to explain the limit", stderr)
	}

	if code, _, _ := runCLI(t, "-base", a, "-current", a, "-max-pixels", "0"); code != exitOK {
		t.Errorf("exit code = %d with the check disabled, want %d", code, exitOK)
	}
}
