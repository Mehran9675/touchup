// Command icon draws TouchUp's app icon: a shape in coarse pixels on the
// left of a compare divider and smooth on the right, the before and after
// of an upscale.
//
//	go run ./tools/icon assets/icon.png
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

const (
	size = 1024
	ss   = 3 // supersampling per axis
)

type rgba struct{ r, g, b, a float64 }

func over(dst, src rgba) rgba {
	a := src.a + dst.a*(1-src.a)
	if a == 0 {
		return rgba{}
	}
	mix := func(d, s float64) float64 { return (s*src.a + d*dst.a*(1-src.a)) / a }
	return rgba{mix(dst.r, src.r), mix(dst.g, src.g), mix(dst.b, src.b), a}
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func hex(v uint32) rgba {
	return rgba{float64(v>>16&0xff) / 255, float64(v>>8&0xff) / 255, float64(v&0xff) / 255, 1}
}

func mixc(a, b rgba, t float64) rgba {
	t = math.Max(0, math.Min(1, t))
	return rgba{lerp(a.r, b.r, t), lerp(a.g, b.g, t), lerp(a.b, b.b, t), lerp(a.a, b.a, t)}
}

// Geometry, in 1024px icon space.
const (
	inset   = 72.0 // tile margin
	cx, cy  = 512.0, 512.0
	radius  = 286.0 // subject disc
	block   = 96.0  // pixel size on the "before" side
	gap     = 10.0
	splitX  = 512.0
	lineW   = 13.0
	handleR = 58.0
)

var (
	violet = hex(0x7C5CFF)
	pink   = hex(0xEC4899)
	amber  = hex(0xFBBF24)
)

// inTile reports whether p is inside the rounded tile (a superellipse,
// like platform icon masks).
func inTile(x, y float64) bool {
	h := (size - 2*inset) / 2
	dx, dy := math.Abs(x-cx)/h, math.Abs(y-cy)/h
	return math.Pow(dx, 5)+math.Pow(dy, 5) <= 1
}

// subject is the color of the smooth disc at p: a violet-pink-amber sweep
// with a soft highlight.
func subject(x, y float64) rgba {
	t := ((x - (cx - radius)) + (y - (cy - radius))) / (4 * radius)
	c := mixc(violet, pink, t*1.6)
	c = mixc(c, amber, (t-0.62)*2.4)
	hl := math.Max(0, 1-math.Hypot(x-(cx-110), y-(cy-130))/230)
	c = mixc(c, rgba{1, 1, 1, 1}, hl*hl*0.45)
	return c
}

func inDisc(x, y float64) bool { return math.Hypot(x-cx, y-cy) <= radius }

func pixel(x, y float64) rgba {
	if !inTile(x, y) {
		return rgba{}
	}
	// Tile: deep ink with a faint violet glow from the top left.
	bg := mixc(hex(0x0B0A12), hex(0x1D1636), 1-math.Hypot(x-250, y-200)/900)
	c := bg
	// Glow behind the subject.
	glow := math.Max(0, 1-math.Hypot(x-cx, y-cy)/(radius*1.55))
	c = over(c, rgba{violet.r, violet.g, violet.b, glow * glow * 0.35})

	if x < splitX {
		// Before: the disc sampled at block centers, drawn as rounded
		// squares with gaps, like an enlarged low-resolution image.
		bx := math.Floor((x-cx)/block)*block + cx
		by := math.Floor((y-cy)/block)*block + cy
		mx, my := bx+block/2, by+block/2
		if inDisc(mx, my) {
			lx, ly := x-bx, y-by
			r := 13.0
			qx := math.Max(math.Abs(lx-block/2)-(block/2-gap/2-r), 0)
			qy := math.Max(math.Abs(ly-block/2)-(block/2-gap/2-r), 0)
			if math.Hypot(qx, qy) <= r {
				s := subject(mx, my)
				s = mixc(s, rgba{0, 0, 0, 1}, 0.12) // a little duller than after
				c = over(c, s)
			}
		}
	} else if inDisc(x, y) {
		c = over(c, subject(x, y))
	}

	// Divider: a white line with a round handle and two chevrons.
	if math.Abs(x-splitX) <= lineW/2 && y > cy-radius-70 && y < cy+radius+70 {
		c = over(c, rgba{1, 1, 1, 0.96})
	}
	d := math.Hypot(x-cx, y-cy)
	if d <= handleR+16 {
		c = over(c, rgba{0, 0, 0, 0.25 * (1 - d/(handleR+16))})
	}
	if d <= handleR {
		c = over(c, rgba{1, 1, 1, 1})
		if chevron(x-cx, y-cy) {
			c = over(c, violet)
		}
	}
	return c
}

// chevron draws ‹ › inside the handle, relative to its center.
func chevron(x, y float64) bool {
	const s, t, off = 17.0, 6.5, 13.0
	for _, side := range []float64{-1, 1} {
		px := (x - side*off) * side // mirror so both point outward
		// Distance to the two strokes of ">" from (0,-s) to (s,0) to (0,s).
		if segDist(px, y, 0, -s, s*0.8, 0) <= t || segDist(px, y, s*0.8, 0, 0, s) <= t {
			return true
		}
	}
	return false
}

func segDist(px, py, ax, ay, bx, by float64) float64 {
	vx, vy := bx-ax, by-ay
	t := math.Max(0, math.Min(1, ((px-ax)*vx+(py-ay)*vy)/(vx*vx+vy*vy)))
	return math.Hypot(px-(ax+t*vx), py-(ay+t*vy))
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: go run ./tools/icon out.png")
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			var acc rgba
			for sy := range ss {
				for sx := range ss {
					p := pixel(float64(x)+(float64(sx)+0.5)/ss, float64(y)+(float64(sy)+0.5)/ss)
					acc.r += p.r * p.a
					acc.g += p.g * p.a
					acc.b += p.b * p.a
					acc.a += p.a
				}
			}
			n := float64(ss * ss)
			if acc.a == 0 {
				continue
			}
			img.SetNRGBA(x, y, color.NRGBA{
				uint8(acc.r / acc.a * 255), uint8(acc.g / acc.a * 255), uint8(acc.b / acc.a * 255), uint8(acc.a / n * 255),
			})
		}
	}
	f, err := os.Create(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}
