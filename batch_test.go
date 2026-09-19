package main

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree builds a directory of images. Each entry maps a relative path to the
// colour the image is filled with; a nil spot means no altered pixel.
func tree(t *testing.T, root string, images map[string]color.RGBA) string {
	t.Helper()

	for name, c := range images {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := SavePNG(path, solid(32, 32, c)); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	return root
}

func TestCompareDirsReportsEachOutcome(t *testing.T) {
	dir := t.TempDir()
	baseDir := tree(t, filepath.Join(dir, "base"), map[string]color.RGBA{
		"same.png":        white,
		"changed.png":     white,
		"gone.png":        white,
		"nested/deep.png": white,
	})
	currentDir := tree(t, filepath.Join(dir, "current"), map[string]color.RGBA{
		"same.png":        white,
		"changed.png":     black,
		"added.png":       white,
		"nested/deep.png": white,
	})

	summary, err := CompareDirs(baseDir, currentDir, "", DefaultOptions(), 0)
	if err != nil {
		t.Fatalf("CompareDirs: %v", err)
	}

	got := map[string]Status{}
	for _, r := range summary.Results {
		got[r.Name] = r.Status
	}

	want := map[string]Status{
		"same.png":        StatusMatch,
		"changed.png":     StatusDiff,
		"gone.png":        StatusMissing,
		"added.png":       StatusNew,
		"nested/deep.png": StatusMatch,
	}

	for name, wantStatus := range want {
		if got[name] != wantStatus {
			t.Errorf("%s: status = %v, want %v", name, got[name], wantStatus)
		}
	}

	if summary.Compared != 3 {
		t.Errorf("Compared = %d, want 3 (added and gone are not compared)", summary.Compared)
	}
	// changed.png differs and gone.png is missing; added.png must not count.
	if summary.FailedCount != 2 {
		t.Errorf("FailedCount = %d, want 2", summary.FailedCount)
	}
	if !summary.Failed() {
		t.Error("Failed() = false, want true")
	}
}

// A screenshot added to the current set is not a regression.
func TestCompareDirsNewImageDoesNotFail(t *testing.T) {
	dir := t.TempDir()
	baseDir := tree(t, filepath.Join(dir, "base"), map[string]color.RGBA{"a.png": white})
	currentDir := tree(t, filepath.Join(dir, "current"), map[string]color.RGBA{
		"a.png": white,
		"b.png": white,
	})

	summary, err := CompareDirs(baseDir, currentDir, "", DefaultOptions(), 0)
	if err != nil {
		t.Fatalf("CompareDirs: %v", err)
	}

	if summary.Failed() {
		t.Errorf("a new image failed the run: %d failures", summary.FailedCount)
	}
}

// A baseline screenshot with no counterpart is a regression: the page is gone.
func TestCompareDirsMissingImageFails(t *testing.T) {
	dir := t.TempDir()
	baseDir := tree(t, filepath.Join(dir, "base"), map[string]color.RGBA{
		"a.png": white,
		"b.png": white,
	})
	currentDir := tree(t, filepath.Join(dir, "current"), map[string]color.RGBA{"a.png": white})

	summary, err := CompareDirs(baseDir, currentDir, "", DefaultOptions(), 0)
	if err != nil {
		t.Fatalf("CompareDirs: %v", err)
	}

	if summary.FailedCount != 1 {
		t.Errorf("FailedCount = %d, want 1", summary.FailedCount)
	}
}

func TestCompareDirsWritesDiffsPreservingLayout(t *testing.T) {
	dir := t.TempDir()
	baseDir := tree(t, filepath.Join(dir, "base"), map[string]color.RGBA{
		"top.png":          white,
		"nested/inner.png": white,
		"unchanged.png":    white,
	})
	currentDir := tree(t, filepath.Join(dir, "current"), map[string]color.RGBA{
		"top.png":          black,
		"nested/inner.png": black,
		"unchanged.png":    white,
	})
	outDir := filepath.Join(dir, "diffs")

	summary, err := CompareDirs(baseDir, currentDir, outDir, DefaultOptions(), 0)
	if err != nil {
		t.Fatalf("CompareDirs: %v", err)
	}

	for _, name := range []string{"top.png", "nested/inner.png"} {
		path := filepath.Join(outDir, filepath.FromSlash(name))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("diff for %s was not written: %v", name, err)
		}
	}

	// Identical images must not produce a diff file.
	if _, err := os.Stat(filepath.Join(outDir, "unchanged.png")); err == nil {
		t.Error("a diff was written for an unchanged image")
	}

	for _, r := range summary.Results {
		if r.Status == StatusDiff && r.OutPath == "" {
			t.Errorf("%s: differed but OutPath is empty", r.Name)
		}
	}
}

// A size mismatch fails only its own pair; the rest of the run continues.
func TestCompareDirsSizeMismatchFailsOnlyThatPair(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "base")
	currentDir := filepath.Join(dir, "current")

	if err := SavePNG(filepath.Join(baseDir, "ok.png"), solid(32, 32, white)); err != nil {
		t.Fatal(err)
	}
	if err := SavePNG(filepath.Join(currentDir, "ok.png"), solid(32, 32, white)); err != nil {
		t.Fatal(err)
	}
	if err := SavePNG(filepath.Join(baseDir, "resized.png"), solid(32, 32, white)); err != nil {
		t.Fatal(err)
	}
	if err := SavePNG(filepath.Join(currentDir, "resized.png"), solid(64, 32, white)); err != nil {
		t.Fatal(err)
	}

	summary, err := CompareDirs(baseDir, currentDir, "", DefaultOptions(), 0)
	if err != nil {
		t.Fatalf("CompareDirs: %v", err)
	}

	byName := map[string]PairResult{}
	for _, r := range summary.Results {
		byName[r.Name] = r
	}

	if byName["ok.png"].Status != StatusMatch {
		t.Errorf("ok.png: status = %v, want match", byName["ok.png"].Status)
	}
	if byName["resized.png"].Status != StatusError {
		t.Errorf("resized.png: status = %v, want error", byName["resized.png"].Status)
	}
	if err := byName["resized.png"].Err; err == nil || !strings.Contains(err.Error(), "32x32") {
		t.Errorf("resized.png: error should name both sizes, got %v", err)
	}
	if summary.FailedCount != 1 {
		t.Errorf("FailedCount = %d, want 1", summary.FailedCount)
	}
}

func TestCompareDirsToleranceAppliesPerPair(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "base")
	currentDir := filepath.Join(dir, "current")

	base := solid(32, 32, white)
	current := solid(32, 32, white)
	current.Set(1, 1, black) // 1/1024 = 0.098%

	if err := SavePNG(filepath.Join(baseDir, "a.png"), base); err != nil {
		t.Fatal(err)
	}
	if err := SavePNG(filepath.Join(currentDir, "a.png"), current); err != nil {
		t.Fatal(err)
	}

	strict, err := CompareDirs(baseDir, currentDir, "", DefaultOptions(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strict.Failed() {
		t.Error("with fail-on 0 the run should fail")
	}

	lenient, err := CompareDirs(baseDir, currentDir, "", DefaultOptions(), 0.01)
	if err != nil {
		t.Fatal(err)
	}
	if lenient.Failed() {
		t.Error("with fail-on 0.01 the run should pass")
	}
	if lenient.Results[0].Status != StatusTolerated {
		t.Errorf("status = %v, want tolerated", lenient.Results[0].Status)
	}
	if lenient.DiffPixels != 1 {
		t.Errorf("DiffPixels = %d, want the difference still counted", lenient.DiffPixels)
	}
}

func TestCompareDirsEmptyIsAnError(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "base")
	currentDir := filepath.Join(dir, "current")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(currentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := CompareDirs(baseDir, currentDir, "", DefaultOptions(), 0); err == nil {
		t.Error("expected an error when neither directory holds images")
	}
}

// Non-image files must be ignored rather than treated as broken images.
func TestImagesUnderIgnoresOtherFiles(t *testing.T) {
	dir := t.TempDir()
	if err := SavePNG(filepath.Join(dir, "a.png"), solid(8, 8, white)); err != nil {
		t.Fatal(err)
	}
	if err := SavePNG(filepath.Join(dir, "sub", "b.PNG"), solid(8, 8, white)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"notes.txt", "data.json", ".gitkeep"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	found, err := imagesUnder(dir)
	if err != nil {
		t.Fatalf("imagesUnder: %v", err)
	}

	want := []string{"a.png", "sub/b.PNG"}
	if len(found) != len(want) {
		t.Fatalf("found %v, want %v", found, want)
	}
	for i := range want {
		if found[i] != want[i] {
			t.Errorf("found[%d] = %q, want %q (slash-separated and sorted)", i, found[i], want[i])
		}
	}
}

func TestSummaryReportMentionsEveryPair(t *testing.T) {
	dir := t.TempDir()
	baseDir := tree(t, filepath.Join(dir, "base"), map[string]color.RGBA{
		"same.png":    white,
		"changed.png": white,
		"gone.png":    white,
	})
	currentDir := tree(t, filepath.Join(dir, "current"), map[string]color.RGBA{
		"same.png":    white,
		"changed.png": black,
	})

	summary, err := CompareDirs(baseDir, currentDir, "", DefaultOptions(), 0)
	if err != nil {
		t.Fatal(err)
	}

	report := summary.Report()

	// gone.png is missing, so only two pairs were actually compared, while
	// both the difference and the missing file count as failures.
	for _, want := range []string{"same.png", "changed.png", "gone.png", "2 compared", "2 failed"} {
		if !strings.Contains(report, want) {
			t.Errorf("report is missing %q:\n%s", want, report)
		}
	}

	md := summary.MarkdownSummary()
	if !strings.Contains(md, "| ") || !strings.Contains(md, "changed.png") {
		t.Errorf("markdown summary is not a table listing the pairs:\n%s", md)
	}
}
