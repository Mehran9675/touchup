package pipeline

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mehran9675/touchup/internal/engine"
)

// picture upscales the w×h PNG at src and returns a PNG of the planned
// size. Engines that can't split a picture themselves get pictures over
// their touchup.tileImage limit in overlapping parts, joined with
// feathered seams, so a large photo doesn't need more GPU memory than
// the engine has.
func (o Options) picture(ctx context.Context, ws *workspace, src string, w, h int, report Reporter, stage string, from, to float64) (string, error) {
	sz := o.size(w, h)
	limit := o.tileLimit()
	if limit <= 0 || (sz.Width <= limit && sz.Height <= limit) {
		j := o.job(src, filepath.Join(ws.out, "picture"+o.imageExt()), ws.in, ws.out, report, stage, from, to)
		j.OutputExt = o.imageExt()
		result, err := o.Engine.Run(ctx, o.sized(j, w, h))
		if err != nil || !sz.Exact {
			return result, err
		}
		sized := filepath.Join(ws.out, "picture-sized.png")
		return sized, o.Tools.ConvertImageSized(ctx, result, sized, sz.Width, sz.Height)
	}

	in, err := readPNG(src)
	if err != nil {
		return "", err
	}
	fx, fy := float64(sz.Width)/float64(w), float64(sz.Height)/float64(h)
	tile := max(16, int(float64(limit)/max(fx, fy)))
	overlap := max(8, tile/8)
	xs, ys := starts(w, tile, overlap), starts(h, tile, overlap)
	total := len(xs) * len(ys)
	canvas := image.NewNRGBA(image.Rect(0, 0, sz.Width, sz.Height))
	outX := func(x int) int { return int(math.Round(float64(x) * fx)) }
	outY := func(y int) int { return int(math.Round(float64(y) * fy)) }

	n := 0
	for row, y0 := range ys {
		y1 := min(h, y0+tile)
		for col, x0 := range xs {
			x1 := min(w, x0+tile)
			if err := ctx.Err(); err != nil {
				return "", err
			}
			part := fmt.Sprintf("%s · part %d of %d", stage, n+1, total)
			pfrom := from + (to-from)*float64(n)/float64(total)
			pto := from + (to-from)*float64(n+1)/float64(total)
			report(Update{Stage: part, Frac: pfrom})

			dir := ws.path("part-" + strconv.Itoa(n))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", err
			}
			tileIn := filepath.Join(dir, "in.png")
			crop := image.NewNRGBA(image.Rect(0, 0, x1-x0, y1-y0))
			draw.Draw(crop, crop.Bounds(), in, image.Pt(x0, y0), draw.Src)
			if err := writePNG(tileIn, crop); err != nil {
				return "", err
			}
			ox0, oy0, ox1, oy1 := outX(x0), outY(y0), outX(x1), outY(y1)
			j := o.job(tileIn, filepath.Join(dir, "out"+o.imageExt()), dir, dir, report, part, pfrom, pto)
			j.OutputExt = o.imageExt()
			j = o.sized(j, x1-x0, y1-y0)
			j.Scale, j.OutWidth, j.OutHeight = sz.Scale, ox1-ox0, oy1-oy0
			result, err := o.Engine.Run(ctx, j)
			if err != nil {
				return "", err
			}
			// Each part is brought to exactly its share of the result.
			exact := filepath.Join(dir, "exact.png")
			if err := o.Tools.ConvertImageSized(ctx, result, exact, ox1-ox0, oy1-oy0); err != nil {
				return "", err
			}
			up, err := readPNG(exact)
			if err != nil {
				return "", err
			}
			// Fade in over what the parts to the left and above left.
			left, top := 0, 0
			if col > 0 {
				left = outX(min(w, xs[col-1]+tile)) - ox0
			}
			if row > 0 {
				top = outY(min(h, ys[row-1]+tile)) - oy0
			}
			blend(canvas, up, image.Pt(ox0, oy0), left, top)
			n++
		}
	}
	report(Update{Stage: stage, Frac: to})
	joined := filepath.Join(ws.out, "picture-joined.png")
	return joined, writePNG(joined, canvas)
}

// tileLimit is the engine's touchup.tileImage value: the largest output
// a side made in one run, or 0 for no limit.
func (o Options) tileLimit() int {
	v, err := strconv.Atoi(o.EngineOptions[engine.OptionTileImage])
	if err != nil {
		return 0
	}
	return v
}

// starts returns where tiles of size tile, sharing overlap pixels, begin
// along n pixels; the last ends at the edge.
func starts(n, tile, overlap int) []int {
	if n <= tile {
		return []int{0}
	}
	var out []int
	step := max(1, tile-overlap)
	for x := 0; x+tile < n; x += step {
		out = append(out, x)
	}
	return append(out, n-tile)
}

// blend draws src onto dst at at, fading it in over the first left
// columns and top rows, where an earlier part already drew.
func blend(dst, src *image.NRGBA, at image.Point, left, top int) {
	b := src.Bounds()
	for y := 0; y < b.Dy(); y++ {
		ay := 1.0
		if y < top {
			ay = (float64(y) + 0.5) / float64(top)
		}
		for x := 0; x < b.Dx(); x++ {
			a := ay
			if x < left {
				a *= (float64(x) + 0.5) / float64(left)
			}
			s := src.PixOffset(x, y)
			d := dst.PixOffset(at.X+x, at.Y+y)
			if a >= 1 {
				copy(dst.Pix[d:d+4], src.Pix[s:s+4])
				continue
			}
			for c := 0; c < 4; c++ {
				dst.Pix[d+c] = uint8(math.Round(float64(dst.Pix[d+c])*(1-a) + float64(src.Pix[s+c])*a))
			}
		}
	}
}

func readPNG(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filepath.Base(path), err)
	}
	if n, ok := img.(*image.NRGBA); ok {
		return n, nil
	}
	n := image.NewNRGBA(img.Bounds())
	draw.Draw(n, n.Bounds(), img, img.Bounds().Min, draw.Src)
	return n, nil
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
