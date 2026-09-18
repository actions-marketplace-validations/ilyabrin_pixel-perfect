package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"os"
	"strconv"
	"strings"
)

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

	src, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return toRGBA(src), nil
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
