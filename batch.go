package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Status describes the outcome of comparing one pair of images.
type Status int

const (
	// StatusMatch means the images were identical within the threshold.
	StatusMatch Status = iota
	// StatusDiff means they differed beyond the allowed ratio.
	StatusDiff
	// StatusTolerated means they differed, but within the allowed ratio.
	StatusTolerated
	// StatusMissing means the baseline has an image the current set lacks.
	StatusMissing
	// StatusNew means the current set has an image the baseline lacks.
	StatusNew
	// StatusError means the pair could not be compared at all.
	StatusError
)

// Failed reports whether this status should fail the run. A new screenshot is
// reported but does not fail: adding a page is not a regression.
func (s Status) Failed() bool {
	switch s {
	case StatusDiff, StatusMissing, StatusError:
		return true
	default:
		return false
	}
}

func (s Status) String() string {
	switch s {
	case StatusMatch:
		return "ok"
	case StatusDiff:
		return "FAIL"
	case StatusTolerated:
		return "ok"
	case StatusMissing:
		return "MISSING"
	case StatusNew:
		return "new"
	default:
		return "ERROR"
	}
}

// PairResult is the outcome for a single image pair.
//
// The diff image itself is written to disk and deliberately not retained: a
// run over fifty 1080p screenshots would otherwise hold hundreds of megabytes
// of RGBA buffers at once.
type PairResult struct {
	// Name is the path relative to the baseline directory, always
	// slash-separated so that output looks the same on every platform.
	Name string

	Status      Status
	DiffPixels  int
	TotalPixels int

	// OutPath is where the diff image was written, empty when none was.
	OutPath string

	Err error
}

// Ratio is the fraction of pixels that differ, in the range [0,1].
func (p PairResult) Ratio() float64 {
	if p.TotalPixels == 0 {
		return 0
	}
	return float64(p.DiffPixels) / float64(p.TotalPixels)
}

// Summary aggregates the results of a whole run.
type Summary struct {
	Results []PairResult

	Compared    int // pairs actually compared
	FailedCount int
	DiffPixels  int // summed over compared pairs
	TotalPixels int
	MaxRatio    float64
}

// Failed reports whether the run should exit non-zero.
func (s Summary) Failed() bool { return s.FailedCount > 0 }

var imageExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
}

// imagesUnder returns every image below dir, as slash-separated paths relative
// to dir, sorted so that output order is stable.
func imagesUnder(dir string) ([]string, error) {
	var found []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !imageExtensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		found = append(found, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(found)
	return found, nil
}

// IsDir reports whether path exists and is a directory.
func IsDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// CompareDirs compares every image in baseDir against its namesake in
// currentDir, writing diff images under outDir when one is given.
//
// Pairs are compared one at a time: each comparison already saturates the CPU
// across its own worker bands, and a stable output order is worth more here
// than overlapping the decodes.
func CompareDirs(baseDir, currentDir, outDir string, opts Options, failOn float64) (Summary, error) {
	baseImages, err := imagesUnder(baseDir)
	if err != nil {
		return Summary{}, fmt.Errorf("reading %s: %w", baseDir, err)
	}

	currentImages, err := imagesUnder(currentDir)
	if err != nil {
		return Summary{}, fmt.Errorf("reading %s: %w", currentDir, err)
	}

	if len(baseImages) == 0 && len(currentImages) == 0 {
		return Summary{}, fmt.Errorf("no images found in %s or %s", baseDir, currentDir)
	}

	inCurrent := make(map[string]bool, len(currentImages))
	for _, name := range currentImages {
		inCurrent[name] = true
	}

	var summary Summary

	for _, name := range baseImages {
		if !inCurrent[name] {
			summary.Results = append(summary.Results, PairResult{
				Name:   name,
				Status: StatusMissing,
			})
			continue
		}

		res := comparePair(
			filepath.Join(baseDir, filepath.FromSlash(name)),
			filepath.Join(currentDir, filepath.FromSlash(name)),
			diffPath(outDir, name),
			name, opts, failOn,
		)
		summary.Results = append(summary.Results, res)
	}

	// Anything only in the current set is reported, never compared.
	inBase := make(map[string]bool, len(baseImages))
	for _, name := range baseImages {
		inBase[name] = true
	}
	for _, name := range currentImages {
		if !inBase[name] {
			summary.Results = append(summary.Results, PairResult{
				Name:   name,
				Status: StatusNew,
			})
		}
	}

	sort.Slice(summary.Results, func(i, j int) bool {
		return summary.Results[i].Name < summary.Results[j].Name
	})

	for _, r := range summary.Results {
		if r.Status.Failed() {
			summary.FailedCount++
		}
		if r.Status == StatusMatch || r.Status == StatusDiff || r.Status == StatusTolerated {
			summary.Compared++
			summary.DiffPixels += r.DiffPixels
			summary.TotalPixels += r.TotalPixels
			if ratio := r.Ratio(); ratio > summary.MaxRatio {
				summary.MaxRatio = ratio
			}
		}
	}

	return summary, nil
}

// diffPath maps a relative image name to its diff destination, or "" when no
// output directory was requested.
func diffPath(outDir, name string) string {
	if outDir == "" {
		return ""
	}
	return filepath.Join(outDir, filepath.FromSlash(name))
}

// comparePair compares one pair and writes its diff image when they differ.
//
// A size mismatch fails this pair rather than the whole run: in a batch it
// usually means the page grew, which is a regression worth reporting next to
// the others, not a usage error.
func comparePair(basePath, currentPath, outPath, name string, opts Options, failOn float64) PairResult {
	out := PairResult{Name: name}

	baseImg, err := LoadRGBA(basePath)
	if err != nil {
		out.Status, out.Err = StatusError, err
		return out
	}

	currentImg, err := LoadRGBA(currentPath)
	if err != nil {
		out.Status, out.Err = StatusError, err
		return out
	}

	res, err := Compare(baseImg, currentImg, opts)
	if err != nil {
		out.Status = StatusError
		if errors.Is(err, ErrSizeMismatch) {
			out.Err = fmt.Errorf("%w (%dx%d against %dx%d)", err,
				baseImg.Bounds().Dx(), baseImg.Bounds().Dy(),
				currentImg.Bounds().Dx(), currentImg.Bounds().Dy())
		} else {
			out.Err = err
		}
		return out
	}

	out.DiffPixels = res.DiffPixels
	out.TotalPixels = res.TotalPixels

	switch {
	case res.Equal():
		out.Status = StatusMatch
	case out.Ratio() > failOn:
		out.Status = StatusDiff
	default:
		out.Status = StatusTolerated
	}

	if outPath != "" && !res.Equal() {
		if err := SavePNG(outPath, res.Image); err != nil {
			out.Status, out.Err = StatusError, err
			return out
		}
		out.OutPath = outPath
	}

	return out
}

// Report renders the per-pair lines followed by a one-line total.
func (s Summary) Report() string {
	var b strings.Builder

	width := 0
	for _, r := range s.Results {
		if len(r.Name) > width {
			width = len(r.Name)
		}
	}

	for _, r := range s.Results {
		fmt.Fprintf(&b, "%-7s %-*s  ", r.Status, width, r.Name)

		switch r.Status {
		case StatusMissing:
			b.WriteString("not in the current set")
		case StatusNew:
			b.WriteString("not in the baseline")
		case StatusError:
			fmt.Fprintf(&b, "%v", r.Err)
		default:
			fmt.Fprintf(&b, "%d/%d pixels (%.4f%%)", r.DiffPixels, r.TotalPixels, r.Ratio()*100)
			if r.OutPath != "" {
				fmt.Fprintf(&b, " -> %s", r.OutPath)
			}
		}
		b.WriteByte('\n')
	}

	fmt.Fprintf(&b, "\n%d compared, %d failed\n", s.Compared, s.FailedCount)
	return b.String()
}

// MarkdownSummary renders the run as a table for the Actions run page.
func (s Summary) MarkdownSummary() string {
	var b strings.Builder

	status := "passed"
	if s.Failed() {
		status = "failed"
	}
	fmt.Fprintf(&b, "### Pixel Perfect: %s\n\n", status)
	fmt.Fprintf(&b, "%d compared, %d failed\n\n", s.Compared, s.FailedCount)

	b.WriteString("| | Image | Difference |\n|---|---|---|\n")
	for _, r := range s.Results {
		detail := ""
		switch r.Status {
		case StatusMissing:
			detail = "not in the current set"
		case StatusNew:
			detail = "not in the baseline"
		case StatusError:
			detail = fmt.Sprintf("%v", r.Err)
		default:
			detail = fmt.Sprintf("%d/%d (%.4f%%)", r.DiffPixels, r.TotalPixels, r.Ratio()*100)
		}
		fmt.Fprintf(&b, "| %s | `%s` | %s |\n", r.Status, r.Name, detail)
	}

	return b.String()
}
