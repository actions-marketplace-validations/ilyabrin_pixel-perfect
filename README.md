# Pixel Perfect

Fast pixel-by-pixel image comparison for visual regression testing.

A single static binary with no dependencies, usable as a CLI or as a GitHub Action.

## What it does

Compares two images of the same size, reports how many pixels differ, and writes
a copy of the baseline with every difference highlighted. In CI it fails the
build when the difference exceeds a tolerance you set.

## Install

```shell
go install github.com/ilyabrin/pixel-perfect@latest
```

The installed binary is named `pixel-perfect`. Examples below use `pp`; rename
it or alias it if you want the short form.

Or build from source:

```shell
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o pp .
```

## Usage

One pair at a time:

```shell
pp -base baseline.png -current screenshot.png -out diff.png
```

```text
1934/2073600 pixels differ (0.0933%)
diff written to diff.png
```

Or a whole directory, which is what a real visual regression run looks like:

```shell
pp -base baseline/ -current screenshots/ -out diffs/
```

```text
ok      checkout.png   0/2073600 pixels (0.0000%)
FAIL    dashboard.png  9024/2073600 pixels (0.4352%) -> diffs/dashboard.png
MISSING pricing.png    not in the current set
new     settings.png   not in the baseline

3 compared, 2 failed
```

Images are matched by their path relative to each directory, so nested
folders line up, and diff images keep the same layout under `-out`.

A baseline screenshot with no counterpart fails the run: the page it covered
is gone. A screenshot that only exists in the current set is reported but does
not fail, since adding a page is not a regression.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-base` | required | Baseline image, or a directory of them |
| `-current` | required | Image under test, or a directory of them |
| `-out` | none | Where to write diffs; a directory when comparing directories. Skipped for images that match |
| `-threshold` | `0` | Max per-channel delta (0-255) still counted as equal |
| `-fail-on` | `0` | Exit 1 when the diff ratio exceeds this fraction (0-1) |
| `-color` | `FA0000` | Highlight colour as RRGGBB |
| `-alpha` | `200` | Highlight opacity (0-255) |
| `-quiet` | `false` | Print nothing, signal the result via exit code |

PNG and JPEG inputs are supported. Output is always PNG.

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Images match within tolerance |
| `1` | Difference exceeded `-fail-on` |
| `2` | Bad usage, unreadable file, or size mismatch |

### Tolerance

Font rendering and antialiasing are not byte-stable between runs, so an exact
match is often too strict in CI. Two knobs relax it:

* `-threshold` ignores small per-channel deltas, which covers antialiasing.
* `-fail-on` ignores a small fraction of differing pixels overall.

```shell
pp -base a.png -current b.png -threshold 8 -fail-on 0.001
```

## GitHub Action

```yaml
- uses: ilyabrin/pixel-perfect@v1
  id: pixel
  with:
    base: baseline/
    current: screenshots/
    out: diffs/
    threshold: 8
    fail-on: 0.001

- uses: actions/upload-artifact@v7
  if: failure()
  with:
    name: visual-diff
    path: diffs/
```

Outputs: `diff-pixels`, `diff-ratio`, `failed`, `compared`, `failed-count`. The action also writes a summary
to the workflow run page.

The action runs a prebuilt image from `ghcr.io/ilyabrin/pixel-perfect`, so the
step starts in a couple of seconds instead of rebuilding the tool from source
on every run.

Taking the screenshots is out of scope. Use Playwright or Puppeteer for that and
hand the files to this action.

## Benchmarks

Measured on an Intel i5-12400F, 8 workers, 1920x1080 RGBA images.
Reproduce with `go test -run '^$' -bench . -benchtime 20x`.

| Case | Time | Throughput |
|------|------|------------|
| Identical images | 1.94 ms | 4266 MB/s |
| Sparse diff (one 200x100 region) | 1.93 ms | 4287 MB/s |
| Screenshot-like, flat colours | 2.04 ms | 4072 MB/s |
| Worst case, every pixel differs | 2.92 ms | 2839 MB/s |

Scaling across workers, worst case:

| Workers | Time | Speedup |
|---------|------|---------|
| 1 | 7.11 ms | 1.0x |
| 2 | 4.86 ms | 1.5x |
| 4 | 3.13 ms | 2.3x |
| 8 | 2.89 ms | 2.5x |

Scaling flattens past four workers because the job is memory-bound rather than
compute-bound: at 2.8 GB/s the comparison is limited by how fast the pixels can
be read, not by how fast they can be compared.

### Against the previous implementation

The original version split both images into 100x100 tiles, PNG-encoded every
tile on both sides to compare MD5 sums, then walked the surviving tiles through
the `image.Image` interface. Both steps are benchmarked in `bench_test.go` so
the comparison stays honest:

| Case | Before | After | Speedup |
|------|--------|-------|---------|
| Identical, photographic content | 610 ms | 1.94 ms | 314x |
| Identical, flat screenshot content | 103 ms | 2.04 ms | 51x |
| Every pixel differs | 604 ms | 2.92 ms | 207x |

Allocations dropped from 395 MB and 14537 allocations per run to 8.3 MB and 34,
where 8.29 MB is the output image itself.

Three changes account for it:

1. **No PNG encoding.** Hashing tiles meant deflating every tile twice per run.
   Comparing the raw `Pix` bytes with `bytes.Equal` is the same test without the
   compression, and it uses SIMD-backed assembly.
2. **No interface calls per pixel.** `img.At(x, y).RGBA()` boxes a `color.Color`
   for every pixel. Decoding once into `*image.RGBA` and indexing `Pix` directly
   removes both the call and the allocation.
3. **Bands instead of tiles.** Work is split into one horizontal band per CPU.
   Bands do not overlap in the output buffer, so there is no mutex and no
   channel, and identical rows are skipped with a single `bytes.Equal`.

The flat-screenshot row is the fair one to quote: photographic noise is the
worst case for a PNG encoder, which flatters the new code. Real screenshots look
more like the 51x row.

## Development

```shell
go test -race -cover ./...      # 90% coverage
go test -run '^$' -bench .      # benchmarks
go vet ./...
```

The comparison core is in `compare.go` and is a pure function: it does not
mutate its inputs and does not touch the filesystem, so every case is testable
with images built in memory.

## License

MIT
