package main

import (
	"image"
	"image/color"
	"math"
)

// 菜单使用同一组轻量线条图形，按实际像素绘制，避免字体符号和放大小图带来的模糊。
func renderMenuGlyph(size int, name string, paint color.NRGBA) *image.NRGBA {
	paths := map[string][][4]float64{
		"check":    {{3, 8, 6.5, 11.5}, {6.5, 11.5, 13, 4.5}},
		"power":    {{8, 2, 8, 7}},
		"settings": {{2, 4, 14, 4}, {2, 8, 14, 8}, {2, 12, 14, 12}, {5, 2, 5, 6}, {11, 6, 11, 10}, {6, 10, 6, 14}},
		"search":   {{10, 10, 14, 14}},
		"network":  {{8, 3, 8, 8}, {3, 8, 13, 8}, {3, 8, 3, 12}, {13, 8, 13, 12}},
		"terminal": {{3, 4, 6, 7}, {6, 7, 3, 10}, {8, 11, 13, 11}},
		"exit":     {{7, 3, 3, 3}, {3, 3, 3, 13}, {3, 13, 7, 13}, {7, 8, 14, 8}, {11, 5, 14, 8}, {14, 8, 11, 11}},
		"speed":    {{8, 9, 12, 5}, {3, 13, 13, 13}},
		"refresh":  {{12, 2, 12, 5}, {12, 5, 15, 5}},
		"layers":   {{2, 6, 8, 3}, {8, 3, 14, 6}, {14, 6, 8, 9}, {8, 9, 2, 6}, {2, 9, 8, 12}, {8, 12, 14, 9}},
		"chevron":  {{6, 3, 11, 8}, {11, 8, 6, 13}},
	}
	return sampleIcon(size, func(x, y float64) color.NRGBA {
		x, y = x*16, y*16
		distance := math.Inf(1)
		for _, line := range paths[name] {
			distance = math.Min(distance, segmentDistance(x, y, line[0], line[1], line[2], line[3]))
		}
		switch name {
		case "profile":
			if math.Hypot(x-8, y-8) <= 4 {
				return paint
			}
		case "more":
			for _, cx := range []float64{3, 8, 13} {
				if math.Hypot(x-cx, y-8) <= 1.1 {
					return paint
				}
			}
		case "power":
			if !(y < 6 && x > 5 && x < 11) {
				distance = math.Min(distance, math.Abs(math.Hypot(x-8, y-8)-5.5))
			}
		case "search":
			distance = math.Min(distance, math.Abs(math.Hypot(x-6.5, y-6.5)-4.5))
		case "test":
			distance = math.Min(distance, math.Abs(math.Hypot(x-8, y-8)-6))
			for _, line := range paths["check"] {
				distance = math.Min(distance, segmentDistance(x, y, line[0], line[1], line[2], line[3]))
			}
		case "speed":
			if y < 10 {
				distance = math.Min(distance, math.Abs(math.Hypot(x-8, y-9)-6))
			}
		case "refresh":
			if !(x > 9 && y < 5) {
				distance = math.Min(distance, math.Abs(math.Hypot(x-8, y-8)-5.5))
			}
		}
		if distance <= .65 {
			return paint
		}
		return color.NRGBA{}
	})
}
