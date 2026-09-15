package plot

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

var (
	colBg     = color.RGBA{247, 247, 247, 255}
	colCard   = color.RGBA{255, 255, 255, 255}
	colAxis   = color.RGBA{210, 210, 210, 255}
	colMuted  = color.RGBA{120, 120, 120, 255}
	colLive   = color.RGBA{34, 34, 34, 255}
	colShadow = color.RGBA{140, 140, 140, 255}
	colInk    = color.RGBA{32, 32, 32, 255}
)

// EquityPNG draws Live vs Shadow-ls5 on a light grayscale chart.
func EquityPNG(live, shadow []float64) ([]byte, error) {
	const W, H = 900, 480
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	fill(img, img.Bounds(), colBg)
	fill(img, image.Rect(24, 24, W-24, H-24), colCard)

	drawString(img, 40, 36, "LIVE vs SHADOW LS5", colInk)
	fill(img, image.Rect(700, 30, 712, 42), colLive)
	drawString(img, 718, 36, "LIVE", colInk)
	fill(img, image.Rect(770, 30, 782, 42), colShadow)
	drawString(img, 788, 36, "SHADOW", colMuted)

	n := len(live)
	if len(shadow) < n {
		n = len(shadow)
	}
	if n < 2 {
		drawString(img, 40, 240, "NO CURVE YET", colMuted)
		return encode(img)
	}
	live, shadow = downsample(live[:n], shadow[:n], 280)

	minV, maxV := live[0], live[0]
	for _, xs := range [][]float64{live, shadow} {
		for _, v := range xs {
			if v < minV {
				minV = v
			}
			if v > maxV {
				maxV = v
			}
		}
	}
	if maxV <= minV {
		maxV = minV + 1
	}
	pad := (maxV - minV) * 0.08
	minV -= pad
	maxV += pad

	left, right := 80, W-40
	top, bottom := 72, H-48
	for i := 0; i <= 4; i++ {
		y := top + (bottom-top)*i/4
		hline(img, left, right, y, colAxis)
		val := maxV - (maxV-minV)*float64(i)/4
		drawString(img, 32, y-4, compactUSD(val), colMuted)
	}

	polyline(img, live, minV, maxV, left, right, top, bottom, colLive, 2)
	polyline(img, shadow, minV, maxV, left, right, top, bottom, colShadow, 2)
	return encode(img)
}

func downsample(a, b []float64, max int) ([]float64, []float64) {
	n := len(a)
	if n <= max {
		return a, b
	}
	outA := make([]float64, max)
	outB := make([]float64, max)
	for i := 0; i < max; i++ {
		j := int(float64(i) * float64(n-1) / float64(max-1))
		outA[i] = a[j]
		outB[i] = b[j]
	}
	return outA, outB
}

func polyline(img *image.RGBA, xs []float64, minV, maxV float64, left, right, top, bottom int, c color.RGBA, width int) {
	n := len(xs)
	if n < 2 {
		return
	}
	spanX := float64(right - left)
	spanY := float64(bottom - top)
	rng := maxV - minV
	px := func(i int) (int, int) {
		x := left + int(float64(i)/float64(n-1)*spanX)
		y := bottom - int((xs[i]-minV)/rng*spanY)
		return x, y
	}
	x0, y0 := px(0)
	for i := 1; i < n; i++ {
		x1, y1 := px(i)
		line(img, x0, y0, x1, y1, c, width)
		x0, y0 = x1, y1
	}
}

func line(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA, w int) {
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy
	for {
		dot(img, x0, y0, c, w)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

func dot(img *image.RGBA, x, y int, c color.RGBA, w int) {
	r := w / 2
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			img.Set(x+dx, y+dy, c)
		}
	}
}

func hline(img *image.RGBA, x0, x1, y int, c color.RGBA) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	for x := x0; x <= x1; x++ {
		img.Set(x, y, c)
	}
}

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.Set(x, y, c)
		}
	}
}

// 5x7 glyphs for A-Z, 0-9 and a few symbols.
var glyph = map[rune][7]string{
	' ': {"00000", "00000", "00000", "00000", "00000", "00000", "00000"},
	'-': {"00000", "00000", "00000", "11111", "00000", "00000", "00000"},
	'.': {"00000", "00000", "00000", "00000", "00000", "01100", "01100"},
	'$': {"01110", "10101", "10100", "01110", "00101", "10101", "01110"},
	'0': {"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "11110", "00001", "00001", "10001", "01110"},
	'6': {"01110", "10000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
	'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
	'D': {"11110", "10001", "10001", "10001", "10001", "10001", "11110"},
	'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
	'H': {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
	'I': {"01110", "00100", "00100", "00100", "00100", "00100", "01110"},
	'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'N': {"10001", "11001", "10101", "10011", "10001", "10001", "10001"},
	'O': {"01110", "10001", "10001", "10001", "10001", "10001", "01110"},
	'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	'S': {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
	'V': {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
	'W': {"10001", "10001", "10001", "10101", "10101", "10101", "01010"},
	'Y': {"10001", "10001", "01010", "00100", "00100", "00100", "00100"},
	'K': {"10001", "10010", "10100", "11000", "10100", "10010", "10001"},
	'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'C': {"01110", "10001", "10000", "10000", "10000", "10001", "01110"},
	'U': {"10001", "10001", "10001", "10001", "10001", "10001", "01110"},
}

func drawString(img *image.RGBA, x, y int, s string, c color.RGBA) {
	cx := x
	for _, r := range s {
		g, ok := glyph[r]
		if !ok {
			g = glyph[' ']
		}
		for row := 0; row < 7; row++ {
			for col := 0; col < 5; col++ {
				if g[row][col] == '1' {
					img.Set(cx+col, y+row, c)
				}
			}
		}
		cx += 6
	}
}

func compactUSD(n float64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	switch {
	case n >= 1_000_000:
		return sign + "$" + trim1(n/1_000_000) + "M"
	case n >= 10_000:
		return sign + "$" + itoa(int(math.Round(n/1000))) + "K"
	default:
		return sign + "$" + itoa(int(math.Round(n)))
	}
}

func trim1(v float64) string {
	s := itoa(int(v*10 + 0.5))
	if len(s) == 1 {
		s = "0" + s
	}
	return s[:len(s)-1] + "." + s[len(s)-1:]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func encode(img *image.RGBA) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
