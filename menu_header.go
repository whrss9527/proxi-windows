package main

import (
	"image/color"
	"math"
	"strconv"
	"strings"
)

// 只读状态也要清晰可读；不使用系统禁用文字色。高对比度主题直接采用系统文字和背景色。
type menuHeaderPalette struct {
	background, card, text, accent color.NRGBA
}

func headerPalette(kind, accent string, dark, highContrast bool, systemBackground, systemText color.NRGBA) menuHeaderPalette {
	background, text := color.NRGBA{249, 249, 249, 255}, color.NRGBA{32, 32, 32, 255}
	if dark {
		background, text = color.NRGBA{43, 43, 43, 255}, color.NRGBA{245, 245, 245, 255}
	}
	if highContrast {
		return menuHeaderPalette{systemBackground, systemBackground, systemText, systemText}
	}
	if kind == "brand" {
		return menuHeaderPalette{background, background, text, text}
	}
	base := color.NRGBA{100, 116, 139, 255}
	switch kind {
	case "on":
		base = color.NRGBA{22, 163, 74, 255}
		if value, err := strconv.ParseUint(strings.TrimPrefix(accent, "#"), 16, 32); err == nil && len(accent) == 7 && strings.HasPrefix(accent, "#") {
			base = color.NRGBA{uint8(value >> 16), uint8(value >> 8), uint8(value), 255}
		}
	case "warning":
		base = color.NRGBA{217, 119, 6, 255}
	case "error":
		base = color.NRGBA{220, 38, 38, 255}
	}
	card := mixHeaderColor(background, base, .12)
	readable := readableHeaderColor(base, card)
	return menuHeaderPalette{background, card, readable, readable}
}

func mixHeaderColor(a, b color.NRGBA, ratio float64) color.NRGBA {
	mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*(1-ratio) + float64(y)*ratio)) }
	return color.NRGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), 255}
}

func headerLuminance(c color.NRGBA) float64 {
	linear := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= .04045 {
			return x / 12.92
		}
		return math.Pow((x+.055)/1.055, 2.4)
	}
	return .2126*linear(c.R) + .7152*linear(c.G) + .0722*linear(c.B)
}

func headerContrast(a, b color.NRGBA) float64 {
	x, y := headerLuminance(a), headerLuminance(b)
	return (max(x, y) + .05) / (min(x, y) + .05)
}

func readableHeaderColor(accent, background color.NRGBA) color.NRGBA {
	target := color.NRGBA{0, 0, 0, 255}
	white := color.NRGBA{255, 255, 255, 255}
	if headerContrast(white, background) > headerContrast(target, background) {
		target = white
	}
	for step := 0; step <= 100; step++ {
		candidate := mixHeaderColor(accent, target, float64(step)/100)
		if headerContrast(candidate, background) >= 7 {
			return candidate
		}
	}
	return target
}
