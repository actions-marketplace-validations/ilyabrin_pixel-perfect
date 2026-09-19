package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

const (
	exitOK       = 0
	exitDiff     = 1
	exitBadUsage = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pp", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		base      = fs.String("base", "", "baseline image, or a directory of them (required)")
		current   = fs.String("current", "", "image under test, or a directory of them (required)")
		out       = fs.String("out", "", "where to write diff images; a directory when comparing directories")
		threshold = fs.Uint("threshold", 0, "max per-channel delta (0-255) still counted as equal")
		failOn    = fs.Float64("fail-on", 0, "exit 1 when the diff ratio exceeds this fraction (0-1)")
		hexColor  = fs.String("color", "FA0000", "highlight colour as RRGGBB")
		alpha     = fs.Uint("alpha", 200, "highlight opacity (0-255)")
		ignoreAA  = fs.Bool("ignore-antialiasing", false, "treat antialiased edge pixels as equal")
		quiet     = fs.Bool("quiet", false, "print nothing, signal the result via exit code")
		showVer   = fs.Bool("version", false, "print the version and exit")
	)

	var ignore Regions
	fs.Var(&ignore, "ignore", "exclude a region as x,y,width,height; repeatable, or several separated by ';'")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "pp %s - fast image comparison for visual regression\n\n", version)
		fmt.Fprintf(stderr, "Usage:\n"+
			"  pp -base a.png -current b.png -out diff.png\n"+
			"  pp -base baseline/ -current screenshots/ -out diffs/\n\nFlags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return exitBadUsage
	}

	if *showVer {
		fmt.Fprintln(stdout, version)
		return exitOK
	}

	if *base == "" || *current == "" {
		fmt.Fprintln(stderr, "error: -base and -current are required")
		fs.Usage()
		return exitBadUsage
	}

	if *threshold > 255 || *alpha > 255 {
		fmt.Fprintln(stderr, "error: -threshold and -alpha must be within 0-255")
		return exitBadUsage
	}

	if *failOn < 0 || *failOn > 1 {
		fmt.Fprintln(stderr, "error: -fail-on must be within 0-1")
		return exitBadUsage
	}

	highlight, err := ParseHexColor(*hexColor, uint8(*alpha))
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitBadUsage
	}

	opts := DefaultOptions()
	opts.Threshold = uint8(*threshold)
	opts.Highlight = highlight
	opts.IgnoreAntialiasing = *ignoreAA
	opts.Ignore = ignore

	baseIsDir, err := IsDir(*base)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitBadUsage
	}

	currentIsDir, err := IsDir(*current)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitBadUsage
	}

	if baseIsDir != currentIsDir {
		fmt.Fprintln(stderr, "error: -base and -current must both be files or both be directories")
		return exitBadUsage
	}

	if baseIsDir {
		return runBatch(*base, *current, *out, opts, *failOn, *quiet, stdout, stderr)
	}

	baseImg, err := LoadRGBA(*base)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitBadUsage
	}

	currentImg, err := LoadRGBA(*current)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitBadUsage
	}

	res, err := Compare(baseImg, currentImg, opts)
	if err != nil {
		if errors.Is(err, ErrSizeMismatch) {
			fmt.Fprintf(stderr, "error: %v (%s is %s, %s is %s)\n",
				err, *base, dims(baseImg.Bounds().Dx(), baseImg.Bounds().Dy()),
				*current, dims(currentImg.Bounds().Dx(), currentImg.Bounds().Dy()))
		} else {
			fmt.Fprintf(stderr, "error: %v\n", err)
		}
		return exitBadUsage
	}

	if *out != "" && !res.Equal() {
		if err := SavePNG(*out, res.Image); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return exitBadUsage
		}
	}

	failed := res.Ratio() > *failOn

	if !*quiet {
		fmt.Fprintf(stdout, "%d/%d pixels differ (%.4f%%)\n",
			res.DiffPixels, res.TotalPixels, res.Ratio()*100)
		if *out != "" && !res.Equal() {
			fmt.Fprintf(stdout, "diff written to %s\n", *out)
		}
	}

	writeActionOutputs(res, failed, *out)

	if failed {
		return exitDiff
	}
	return exitOK
}

func dims(w, h int) string { return fmt.Sprintf("%dx%d", w, h) }

// runBatch compares two directories of images.
func runBatch(baseDir, currentDir, outDir string, opts Options, failOn float64, quiet bool, stdout, stderr io.Writer) int {
	summary, err := CompareDirs(baseDir, currentDir, outDir, opts, failOn)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitBadUsage
	}

	if !quiet {
		fmt.Fprint(stdout, summary.Report())
	}

	writeBatchOutputs(summary)

	if summary.Failed() {
		return exitDiff
	}
	return exitOK
}

// writeBatchOutputs publishes a directory run to the Actions runner.
func writeBatchOutputs(s Summary) {
	appendFile(os.Getenv("GITHUB_OUTPUT"), fmt.Sprintf(
		"diff-pixels=%d\ndiff-ratio=%.6f\nfailed=%t\ncompared=%d\nfailed-count=%d\n",
		s.DiffPixels, s.MaxRatio, s.Failed(), s.Compared, s.FailedCount))

	appendFile(os.Getenv("GITHUB_STEP_SUMMARY"), s.MarkdownSummary())
}

// writeActionOutputs publishes results to the GitHub Actions runner when one is
// present. Both files are absent outside CI, so failures are ignored.
func writeActionOutputs(res Result, failed bool, out string) {
	failedCount := 0
	if failed {
		failedCount = 1
	}
	appendFile(os.Getenv("GITHUB_OUTPUT"), fmt.Sprintf(
		"diff-pixels=%d\ndiff-ratio=%.6f\nfailed=%t\ncompared=1\nfailed-count=%d\n",
		res.DiffPixels, res.Ratio(), failed, failedCount))

	status := "passed"
	if failed {
		status = "failed"
	}
	summary := fmt.Sprintf("### Pixel Perfect: %s\n\n%d of %d pixels differ (%.4f%%)\n",
		status, res.DiffPixels, res.TotalPixels, res.Ratio()*100)
	if out != "" && !res.Equal() {
		summary += fmt.Sprintf("\nDiff image: `%s`\n", out)
	}
	appendFile(os.Getenv("GITHUB_STEP_SUMMARY"), summary)
}

func appendFile(path, content string) {
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(content)
}
