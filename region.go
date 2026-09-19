package main

import (
	"fmt"
	"image"
	"strconv"
	"strings"
)

// Regions is a list of rectangles excluded from comparison, and a flag.Value
// so that -ignore can be repeated on the command line.
type Regions []image.Rectangle

// String renders the regions back into the flag syntax.
func (r *Regions) String() string {
	if r == nil || len(*r) == 0 {
		return ""
	}

	parts := make([]string, 0, len(*r))
	for _, rect := range *r {
		parts = append(parts, fmt.Sprintf("%d,%d,%d,%d",
			rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy()))
	}
	return strings.Join(parts, " ")
}

// Set parses one flag occurrence. A single occurrence may hold several
// regions separated by semicolons, which is how the action passes a list
// through one input.
func (r *Regions) Set(value string) error {
	for _, spec := range strings.Split(value, ";") {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}

		rect, err := ParseRegion(spec)
		if err != nil {
			return err
		}
		*r = append(*r, rect)
	}
	return nil
}

// ParseRegion reads "x,y,width,height" into a rectangle.
func ParseRegion(spec string) (image.Rectangle, error) {
	fields := strings.Split(spec, ",")
	if len(fields) != 4 {
		return image.Rectangle{}, fmt.Errorf("region %q must be x,y,width,height", spec)
	}

	values := make([]int, 4)
	for i, f := range fields {
		v, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			return image.Rectangle{}, fmt.Errorf("region %q: %q is not a number", spec, f)
		}
		values[i] = v
	}

	x, y, w, h := values[0], values[1], values[2], values[3]
	if w <= 0 || h <= 0 {
		return image.Rectangle{}, fmt.Errorf("region %q: width and height must be positive", spec)
	}
	if x < 0 || y < 0 {
		return image.Rectangle{}, fmt.Errorf("region %q: x and y must not be negative", spec)
	}

	return image.Rect(x, y, x+w, y+h), nil
}

// span is a half-open range of x positions to skip.
//
// Masking is resolved per row rather than per pixel: a row either intersects
// some region or it does not, and most rows do not.
type span struct{ from, to int }

// maskForRow returns the spans of row y covered by any region, or nil when the
// row is untouched.
func maskForRow(regions Regions, y, width int) []span {
	var spans []span

	for _, r := range regions {
		if y < r.Min.Y || y >= r.Max.Y {
			continue
		}

		from, to := r.Min.X, r.Max.X
		if from < 0 {
			from = 0
		}
		if to > width {
			to = width
		}
		if from >= to {
			continue
		}
		spans = append(spans, span{from, to})
	}

	return spans
}

// covers reports whether x falls in any span.
func covers(spans []span, x int) bool {
	for _, s := range spans {
		if x >= s.from && x < s.to {
			return true
		}
	}
	return false
}

// IgnoredPixels counts how many pixels of a width x height image the regions
// cover, without double-counting overlaps.
func (r Regions) IgnoredPixels(width, height int) int {
	if len(r) == 0 {
		return 0
	}

	total := 0
	for y := 0; y < height; y++ {
		spans := maskForRow(r, y, width)
		if len(spans) == 0 {
			continue
		}
		for x := 0; x < width; x++ {
			if covers(spans, x) {
				total++
			}
		}
	}
	return total
}
