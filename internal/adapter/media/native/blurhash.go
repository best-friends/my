// SPDX-License-Identifier: AGPL-3.0-only

package native

import (
	"image"
	"math"
	"strings"
)

// The blurhash encoder follows the reference implementation (https://github.com/woltapp/blurhash,
// MIT): a DCT of the image in linear RGB, quantized and written in base 83.

const base83 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz#$%*+,-.:;=?@[]^_{|}~"

// Blurhash components, as Mastodon uses them.
const (
	blurhashX = 4
	blurhashY = 4
)

// blurhash encodes img with cx × cy components (1 to 9 each).
func blurhash(img *image.NRGBA, cx, cy int) string {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	// Linearize every pixel once.
	lin := make([][3]float64, w*h)
	for y := range h {
		for x := range w {
			i := img.PixOffset(img.Rect.Min.X+x, img.Rect.Min.Y+y)
			lin[y*w+x] = [3]float64{srgbToLinear(img.Pix[i]), srgbToLinear(img.Pix[i+1]), srgbToLinear(img.Pix[i+2])}
		}
	}
	cosX := cosTable(cx, w)
	cosY := cosTable(cy, h)
	factors := make([][3]float64, 0, cx*cy)
	for j := range cy {
		for i := range cx {
			norm := 2.0
			if i == 0 && j == 0 {
				norm = 1
			}
			var f [3]float64
			for y := range h {
				by := norm * cosY[j][y]
				for x := range w {
					basis := by * cosX[i][x]
					p := lin[y*w+x]
					f[0] += basis * p[0]
					f[1] += basis * p[1]
					f[2] += basis * p[2]
				}
			}
			scale := 1 / float64(w*h)
			factors = append(factors, [3]float64{f[0] * scale, f[1] * scale, f[2] * scale})
		}
	}

	var sb strings.Builder
	encode83(&sb, (cx-1)+(cy-1)*9, 1)
	dc, ac := factors[0], factors[1:]
	maxValue := 1.0
	if len(ac) > 0 {
		actualMax := 0.0
		for _, f := range ac {
			actualMax = max(actualMax, math.Abs(f[0]), math.Abs(f[1]), math.Abs(f[2]))
		}
		quantisedMax := int(max(0, min(82, math.Floor(actualMax*166-0.5))))
		maxValue = float64(quantisedMax+1) / 166
		encode83(&sb, quantisedMax, 1)
	} else {
		encode83(&sb, 0, 1)
	}
	encode83(&sb, linearToSRGB(dc[0])<<16+linearToSRGB(dc[1])<<8+linearToSRGB(dc[2]), 4)
	for _, f := range ac {
		q := func(v float64) int {
			return int(max(0, min(18, math.Floor(signPow(v/maxValue, 0.5)*9+9.5))))
		}
		encode83(&sb, q(f[0])*19*19+q(f[1])*19+q(f[2]), 2)
	}
	return sb.String()
}

func cosTable(components, n int) [][]float64 {
	t := make([][]float64, components)
	for c := range components {
		t[c] = make([]float64, n)
		for k := range n {
			t[c][k] = math.Cos(math.Pi * float64(c) * float64(k) / float64(n))
		}
	}
	return t
}

func encode83(sb *strings.Builder, value, length int) {
	for i := 1; i <= length; i++ {
		digit := value / int(math.Pow(83, float64(length-i))) % 83
		sb.WriteByte(base83[digit])
	}
}

func srgbToLinear(v uint8) float64 {
	f := float64(v) / 255
	if f <= 0.04045 {
		return f / 12.92
	}
	return math.Pow((f+0.055)/1.055, 2.4)
}

func linearToSRGB(v float64) int {
	v = max(0, min(1, v))
	if v <= 0.0031308 {
		return int(v*12.92*255 + 0.5)
	}
	return int((1.055*math.Pow(v, 1/2.4)-0.055)*255 + 0.5)
}

func signPow(v, exp float64) float64 {
	return math.Copysign(math.Pow(math.Abs(v), exp), v)
}
