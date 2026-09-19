package main

import "image"

// Antialiasing detection, following the approach pixelmatch uses.
//
// Font rendering and edge smoothing are not stable between runs: the same page
// screenshotted twice can differ along every glyph and curve. A plain
// per-channel threshold cannot separate that from a real change, because an
// antialiased edge pixel and a genuinely recoloured pixel look identical in
// isolation. The difference is in the neighbourhood.
//
// A pixel is treated as antialiasing when, among its eight neighbours, it is
// both darker than some and brighter than others (so it sits on an edge), and
// the extreme neighbour is part of a flat run of identical pixels in both
// images (so that edge exists in both, and only its smoothing moved).

// luminance returns the Y component of YIQ, composited over white when the
// pixel is translucent. Antialiasing shows up as a brightness gradient, so
// only luminance matters here.
func luminance(r, g, b, a uint8) float64 {
	rf, gf, bf := float64(r), float64(g), float64(b)

	if a < 255 {
		// Blend onto white, matching how the screenshot would be viewed.
		af := float64(a) / 255
		rf = 255 + (rf-255)*af
		gf = 255 + (gf-255)*af
		bf = 255 + (bf-255)*af
	}

	return rf*0.29889531 + gf*0.58662247 + bf*0.11448223
}

// luminanceAt reads the luminance of one pixel.
func luminanceAt(img *image.RGBA, x, y int) float64 {
	i := img.PixOffset(x, y)
	return luminance(img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3])
}

// sameColor reports whether two pixels of the same image are identical.
func sameColor(img *image.RGBA, x1, y1, x2, y2 int) bool {
	i := img.PixOffset(x1, y1)
	j := img.PixOffset(x2, y2)
	return img.Pix[i] == img.Pix[j] &&
		img.Pix[i+1] == img.Pix[j+1] &&
		img.Pix[i+2] == img.Pix[j+2] &&
		img.Pix[i+3] == img.Pix[j+3]
}

// hasManySiblings reports whether a pixel has at least three neighbours of its
// own colour, which marks it as part of a flat region rather than an edge.
// Pixels on the image border count their missing neighbours as one sibling.
func hasManySiblings(img *image.RGBA, x, y int) bool {
	b := img.Bounds()
	x0, y0 := max(x-1, b.Min.X), max(y-1, b.Min.Y)
	x2, y2 := min(x+1, b.Max.X-1), min(y+1, b.Max.Y-1)

	siblings := 0
	if x == x0 || x == x2 || y == y0 || y == y2 {
		siblings = 1
	}

	for ny := y0; ny <= y2; ny++ {
		for nx := x0; nx <= x2; nx++ {
			if nx == x && ny == y {
				continue
			}
			if sameColor(img, x, y, nx, ny) {
				siblings++
				if siblings > 2 {
					return true
				}
			}
		}
	}

	return false
}

// isAntialiased reports whether the pixel at (x,y) in img looks like an
// antialiased edge pixel, judged against the same position in other.
func isAntialiased(img, other *image.RGBA, x, y int) bool {
	b := img.Bounds()
	x0, y0 := max(x-1, b.Min.X), max(y-1, b.Min.Y)
	x2, y2 := min(x+1, b.Max.X-1), min(y+1, b.Max.Y-1)

	centre := luminanceAt(img, x, y)

	var (
		minDelta, maxDelta float64
		minX, minY         int
		maxX, maxY         int
		identical          int
	)

	for ny := y0; ny <= y2; ny++ {
		for nx := x0; nx <= x2; nx++ {
			if nx == x && ny == y {
				continue
			}

			delta := centre - luminanceAt(img, nx, ny)

			switch {
			case delta == 0:
				// Three identical neighbours mean a flat area, not an edge.
				identical++
				if identical > 2 {
					return false
				}
			case delta < minDelta:
				minDelta, minX, minY = delta, nx, ny
			case delta > maxDelta:
				maxDelta, maxX, maxY = delta, nx, ny
			}
		}
	}

	// An antialiased pixel lies between a darker and a brighter neighbour.
	if minDelta == 0 || maxDelta == 0 {
		return false
	}

	// The edge must exist in both images: only its smoothing may have moved.
	return (hasManySiblings(img, minX, minY) && hasManySiblings(other, minX, minY)) ||
		(hasManySiblings(img, maxX, maxY) && hasManySiblings(other, maxX, maxY))
}

// antialiasedInEither reports whether the differing pixel at (x,y) is
// explained by antialiasing in one image or the other.
func antialiasedInEither(a, b *image.RGBA, x, y int) bool {
	return isAntialiased(a, b, x, y) || isAntialiased(b, a, x, y)
}
