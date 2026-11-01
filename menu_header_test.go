package main

import (
	"image/color"
	"testing"
)

func TestMenuHeaderReadableInBothThemes(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, kind := range []string{"brand", "on", "off", "warning", "error"} {
			for _, accent := range []string{"#16a34a", "#2563eb", "#a855f7", "#ffffff", "#000000", "#fef08a", "invalid"} {
				palette := headerPalette(kind, accent, dark, false, color.NRGBA{}, color.NRGBA{})
				if ratio := headerContrast(palette.text, palette.card); ratio < 7 {
					t.Errorf("状态文字应有足够对比度：dark=%v kind=%s accent=%s ratio=%.2f", dark, kind, accent, ratio)
				}
				if kind == "on" && accent == "#16a34a" && palette.text.R == palette.text.G {
					t.Error("已连接时应保留配置的绿色，不应变成禁用灰色")
				}
			}
		}
	}
}

func TestMenuHeaderHonorsHighContrast(t *testing.T) {
	background, text := color.NRGBA{0, 0, 0, 255}, color.NRGBA{255, 255, 0, 255}
	for _, kind := range []string{"brand", "on", "off", "warning", "error"} {
		palette := headerPalette(kind, "#16a34a", true, true, background, text)
		if palette.card != background || palette.text != text || palette.accent != text {
			t.Errorf("高对比度时应完整使用系统配色：%s %+v", kind, palette)
		}
	}
}
