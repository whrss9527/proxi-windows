package main

import (
	"image"
	"image/color"
	"math"
)

// 固定的应用标识：蓝紫色圆角底上的 P 形路径与分流端点；不随代理开关变化。
func renderBrandIcon(size int) *image.NRGBA {
	return sampleIcon(size, func(x, y float64) color.NRGBA {
		x, y = x*64, y*64
		if !insideRoundedTile(x, y, 4, 60, 15) {
			return color.NRGBA{}
		}
		ratio := (y - 4) / 56
		paint := color.NRGBA{uint8(68 + 23*ratio), uint8(137 - 60*ratio), uint8(255 - 23*ratio), 255}
		distance := math.Min(segmentDistance(x, y, 22, 47, 22, 18), math.Min(segmentDistance(x, y, 22, 18, 33, 18), segmentDistance(x, y, 22, 36, 33, 36)))
		if x >= 33 {
			distance = math.Min(distance, math.Abs(math.Hypot(x-33, y-27)-9))
		}
		if distance <= 3 {
			paint = iconWhite
		}
		if math.Hypot(x-44, y-46) <= 4.5 {
			paint = color.NRGBA{128, 241, 216, 255}
		}
		return paint
	})
}

func sampleIcon(size int, paint func(float64, float64) color.NRGBA) *image.NRGBA {
	canvas := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a float64
			for sy := 0; sy < iconSupersampling; sy++ {
				for sx := 0; sx < iconSupersampling; sx++ {
					c := paint((float64(x)+(float64(sx)+.5)/iconSupersampling)/float64(size), (float64(y)+(float64(sy)+.5)/iconSupersampling)/float64(size))
					weight := float64(c.A) / 255
					r += float64(c.R) * weight
					g += float64(c.G) * weight
					b += float64(c.B) * weight
					a += weight
				}
			}
			if a > 0 {
				canvas.SetNRGBA(x, y, color.NRGBA{uint8(math.Round(r / a)), uint8(math.Round(g / a)), uint8(math.Round(b / a)), uint8(math.Round(a / (iconSupersampling * iconSupersampling) * 255))})
			}
		}
	}
	return canvas
}

func insideRoundedTile(x, y, low, high, radius float64) bool {
	if x < low || x > high || y < low || y > high {
		return false
	}
	cx := math.Max(low+radius, math.Min(x, high-radius))
	cy := math.Max(low+radius, math.Min(y, high-radius))
	return math.Hypot(x-cx, y-cy) <= radius
}

func segmentDistance(x, y, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	t := math.Max(0, math.Min(1, ((x-ax)*dx+(y-ay)*dy)/(dx*dx+dy*dy)))
	return math.Hypot(x-ax-t*dx, y-ay-t*dy)
}

// 托盘可用应用标识加状态角标，也可保留原来的开关。角标用形状辅助颜色区分状态。
func renderTrayIcon(size int, mode, state, profileColor string) *image.NRGBA {
	if mode == "toggle" {
		return renderToggleIcon(size, trayIconStyle(state, profileColor))
	}
	brand := renderBrandIcon(size)
	style := trayIconStyle(state, profileColor)
	fill := style.Track
	if state == iconStateWarn {
		fill = iconAmber
	}
	badge := sampleIcon(size, func(x, y float64) color.NRGBA {
		d := math.Hypot(x-.78, y-.78)
		if d > .22 {
			return color.NRGBA{}
		}
		if d > .17 {
			return iconWhite
		}
		mark := false
		switch state {
		case iconStateOn:
			mark = math.Min(segmentDistance(x, y, .68, .78, .75, .85), segmentDistance(x, y, .75, .85, .88, .7)) < .025
		case iconStateWarn, iconStateExternal:
			mark = segmentDistance(x, y, .78, .68, .78, .79) < .025 || math.Hypot(x-.78, y-.86) < .028
		case iconStateError:
			mark = math.Min(segmentDistance(x, y, .71, .71, .85, .85), segmentDistance(x, y, .71, .85, .85, .71)) < .025
		default:
			mark = segmentDistance(x, y, .69, .78, .87, .78) < .025
		}
		if mark {
			return iconWhite
		}
		return fill
	})
	// 使用标准的 alpha 合成，保留小尺寸角标边缘的抗锯齿。
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := badge.NRGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			b := brand.NRGBAAt(x, y)
			a := float64(c.A) / 255
			out := a + float64(b.A)/255*(1-a)
			mix := func(f, base uint8) uint8 {
				return uint8(math.Round((float64(f)*a + float64(base)*float64(b.A)/255*(1-a)) / out))
			}
			brand.SetNRGBA(x, y, color.NRGBA{mix(c.R, b.R), mix(c.G, b.G), mix(c.B, b.B), uint8(math.Round(out * 255))})
		}
	}
	return brand
}
