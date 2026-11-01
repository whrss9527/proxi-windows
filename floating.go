package main

import (
	"errors"
	"strings"
)

// 悬浮按钮的位置只属于本机，不参与多机同步。
type FloatingWidgetConfig struct {
	Enabled    bool `json:"enabled"`
	Positioned bool `json:"positioned"`
	X          int  `json:"x"`
	Y          int  `json:"y"`
}

type floatingArea struct{ left, top, right, bottom int }
type floatingGeometry struct{ x, y, width, height int }

func floatingLayout(config FloatingWidgetConfig, work floatingArea, dpi int) floatingGeometry {
	scale := func(v int) int { return (v*max(96, dpi) + 48) / 96 }
	width, height := min(scale(164), max(1, work.right-work.left)), min(scale(44), max(1, work.bottom-work.top))
	x, y := config.X, config.Y
	if !config.Positioned {
		x, y = work.right-width-scale(20), work.bottom-height-scale(72)
	}
	x = max(work.left, min(x, work.right-width))
	y = max(work.top, min(y, work.bottom-height))
	return floatingGeometry{x, y, width, height}
}

func floatingCommandTarget(argument string, current bool) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(argument)) {
	case "on":
		return true, nil
	case "off":
		return false, nil
	case "", "toggle":
		return !current, nil
	default:
		return current, errors.New("悬浮按钮参数应为 on 或 off")
	}
}
