package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"strconv"
	"strings"
)

// DefaultMaxPixels caps how large an image may be before it is refused.
//
// A comparison holds three RGBA buffers at once, at four bytes per pixel, so
// this ceiling corresponds to roughly 1.2 GB of pixel data. It is deliberately
// far above any real screenshot: a full-page capture of 1920x20000 is only 38
// million pixels. The point is to fail with a clear message on a malformed or
// hostile file instead of being killed by the OOM reaper.
const DefaultMaxPixels = 100_000_000

// maxPixels is the active ceiling. Zero disables the check.
var maxPixels = DefaultMaxPixels

// LoadRGBA decodes an image file into an *image.RGBA anchored at (0,0).
//
// Decoding once into a concrete RGBA buffer is what makes the comparison fast:
// everything downstream reads the Pix slice directly instead of going through
// the image.Image interface.
func LoadRGBA(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// The header alone carries the dimensions, so an oversized file is
	// rejected before any pixels are allocated.
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	if err := checkSize(path, cfg.Width, cfg.Height); err != nil {
		return nil, err
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewinding %s: %w", path, err)
	}

	src, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return toRGBA(src), nil
}

// checkSize refuses images too large to hold in memory safely.
func checkSize(path string, width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("%s has no pixels (%dx%d)", path, width, height)
	}

	if maxPixels <= 0 {
		return nil
	}

	if width > maxPixels/height {
		return fmt.Errorf("%s is %dx%d, %d megapixels, over the %d megapixel limit; raise -max-pixels to allow it",
			path, width, height,
			(width*height)/1_000_000, maxPixels/1_000_000)
	}
	return nil
}

func toRGBA(src image.Image) *image.RGBA {
	if rgba, ok := src.(*image.RGBA); ok && rgba.Bounds().Min == (image.Point{}) {
		return rgba
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

// SavePNG writes img to path, creating parent directories as needed.
func SavePNG(path string, img image.Image) error {
	if dir := filepathDir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return err
	}
	return f.Close()
}

// filepathDir returns the directory part of a slash- or backslash-separated
// path, or "" when there is none.
func filepathDir(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i > 0 {
		return path[:i]
	}
	return ""
}

// ParseHexColor converts "RRGGBB" or "#RRGGBB" to an RGBA value with the given
// alpha.
func ParseHexColor(h string, alpha uint8) (color.RGBA, error) {
	h = strings.TrimPrefix(strings.TrimSpace(h), "#")
	if len(h) != 6 {
		return color.RGBA{}, fmt.Errorf("colour %q must be 6 hex digits", h)
	}

	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("colour %q is not valid hex: %w", h, err)
	}

	return color.RGBA{
		R: uint8(v >> 16),
		G: uint8(v >> 8),
		B: uint8(v),
		A: alpha,
	}, nil
}
