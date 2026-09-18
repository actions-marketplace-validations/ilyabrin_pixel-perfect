package main

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePNG saves an image into the test's temp dir and returns its path.
func writePNG(t *testing.T, name string, w, h int, c color.RGBA, spot *[2]int) string {
	t.Helper()

	img := solid(w, h, c)
	if spot != nil {
		img.Set(spot[0], spot[1], black)
	}

	path := filepath.Join(t.TempDir(), name)
	if err := SavePNG(path, img); err != nil {
		t.Fatalf("SavePNG(%s): %v", path, err)
	}
	return path
}

func runCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errBuf bytes.Buffer
	code = run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestRunIdenticalImagesExitsZero(t *testing.T) {
	a := writePNG(t, "a.png", 32, 32, white, nil)
	b := writePNG(t, "b.png", 32, 32, white, nil)

	code, stdout, _ := runCLI(t, "-base", a, "-current", b)

	if code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout, "0/1024 pixels differ") {
		t.Errorf("stdout = %q, want a zero-diff report", stdout)
	}
}

func TestRunDifferingImagesExitsOne(t *testing.T) {
	a := writePNG(t, "a.png", 32, 32, white, nil)
	b := writePNG(t, "b.png", 32, 32, white, &[2]int{5, 5})

	code, stdout, _ := runCLI(t, "-base", a, "-current", b)

	if code != exitDiff {
		t.Errorf("exit code = %d, want %d", code, exitDiff)
	}
	if !strings.Contains(stdout, "1/1024 pixels differ") {
		t.Errorf("stdout = %q, want a one-pixel report", stdout)
	}
}

// A diff below -fail-on must not fail the build: antialiasing noise is normal.
func TestRunToleratesDiffBelowFailOn(t *testing.T) {
	a := writePNG(t, "a.png", 32, 32, white, nil)
	b := writePNG(t, "b.png", 32, 32, white, &[2]int{5, 5})

	code, _, _ := runCLI(t, "-base", a, "-current", b, "-fail-on", "0.01")

	if code != exitOK {
		t.Errorf("exit code = %d, want %d: 1/1024 is below the 1%% threshold", code, exitOK)
	}
}

func TestRunWritesDiffImage(t *testing.T) {
	a := writePNG(t, "a.png", 32, 32, white, nil)
	b := writePNG(t, "b.png", 32, 32, white, &[2]int{5, 5})
	out := filepath.Join(t.TempDir(), "nested", "diff.png")

	if code, _, _ := runCLI(t, "-base", a, "-current", b, "-out", out); code != exitDiff {
		t.Fatalf("exit code = %d, want %d", code, exitDiff)
	}

	// The parent directory must be created rather than reported as an error.
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("diff image was not written: %v", err)
	}

	written, err := LoadRGBA(out)
	if err != nil {
		t.Fatalf("diff image is not readable: %v", err)
	}
	if got := written.RGBAAt(5, 5); got == white {
		t.Error("the differing pixel is not highlighted in the written diff")
	}
}

func TestRunSizeMismatchReportsBothSizes(t *testing.T) {
	a := writePNG(t, "a.png", 32, 32, white, nil)
	b := writePNG(t, "b.png", 32, 48, white, nil)

	code, _, stderr := runCLI(t, "-base", a, "-current", b)

	if code != exitBadUsage {
		t.Errorf("exit code = %d, want %d", code, exitBadUsage)
	}
	for _, want := range []string{"32x32", "32x48"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want it to mention %s", stderr, want)
		}
	}
}

func TestRunRejectsBadUsage(t *testing.T) {
	a := writePNG(t, "a.png", 8, 8, white, nil)

	for name, args := range map[string][]string{
		"no arguments":    {},
		"missing current": {"-base", a},
		"missing file":    {"-base", a, "-current", "does-not-exist.png"},
		"bad colour":      {"-base", a, "-current", a, "-color", "ZZZ"},
		"fail-on above 1": {"-base", a, "-current", a, "-fail-on", "1.5"},
		"alpha too large": {"-base", a, "-current", a, "-alpha", "300"},
	} {
		if code, _, _ := runCLI(t, args...); code != exitBadUsage {
			t.Errorf("%s: exit code = %d, want %d", name, code, exitBadUsage)
		}
	}
}

func TestRunQuietPrintsNothing(t *testing.T) {
	a := writePNG(t, "a.png", 8, 8, white, nil)

	if _, stdout, _ := runCLI(t, "-base", a, "-current", a, "-quiet"); stdout != "" {
		t.Errorf("stdout = %q, want it empty under -quiet", stdout)
	}
}

func TestRunWritesGitHubActionsOutputs(t *testing.T) {
	a := writePNG(t, "a.png", 32, 32, white, nil)
	b := writePNG(t, "b.png", 32, 32, white, &[2]int{5, 5})

	dir := t.TempDir()
	outputFile := filepath.Join(dir, "output")
	summaryFile := filepath.Join(dir, "summary")
	t.Setenv("GITHUB_OUTPUT", outputFile)
	t.Setenv("GITHUB_STEP_SUMMARY", summaryFile)

	runCLI(t, "-base", a, "-current", b, "-quiet")

	output, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("GITHUB_OUTPUT was not written: %v", err)
	}
	for _, want := range []string{"diff-pixels=1", "diff-ratio=0.000977", "failed=true"} {
		if !strings.Contains(string(output), want) {
			t.Errorf("GITHUB_OUTPUT = %q, want it to contain %q", output, want)
		}
	}

	summary, err := os.ReadFile(summaryFile)
	if err != nil {
		t.Fatalf("GITHUB_STEP_SUMMARY was not written: %v", err)
	}
	if !strings.Contains(string(summary), "failed") {
		t.Errorf("summary = %q, want it to report a failure", summary)
	}
}

func TestRunVersion(t *testing.T) {
	code, stdout, _ := runCLI(t, "-version")

	if code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
	if strings.TrimSpace(stdout) != version {
		t.Errorf("stdout = %q, want %q", stdout, version)
	}
}

func TestLoadRGBARejectsNonImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-an-image.png")
	if err := os.WriteFile(path, []byte("this is not a png"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadRGBA(path); err == nil {
		t.Error("expected an error when decoding a non-image file")
	}
}
