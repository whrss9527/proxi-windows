package main

import "testing"

func TestFloatingLayoutStaysOnScreen(t *testing.T) {
	for _, dpi := range []int{96, 144, 192} {
		for _, work := range []floatingArea{{0, 0, 1920, 1040}, {-1920, 0, 0, 1040}, {0, -1080, 1600, 0}, {0, 0, 100, 40}} {
			for _, config := range []FloatingWidgetConfig{{}, {Positioned: true, X: 99999, Y: -99999}, {Positioned: true, X: -1500, Y: 500}} {
				box := floatingLayout(config, work, dpi)
				if box.x < work.left || box.y < work.top || box.x+box.width > work.right || box.y+box.height > work.bottom {
					t.Errorf("悬浮按钮不能掉到屏幕外：%+v %+v", box, work)
				}
			}
		}
	}
	box := floatingLayout(FloatingWidgetConfig{Positioned: true, X: -1500, Y: 500}, floatingArea{-1920, 0, 0, 1040}, 96)
	if box.x != -1500 || box.y != 500 {
		t.Error("左侧显示器上的负坐标也要记住")
	}
}

func TestFloatingCommandAndConfig(t *testing.T) {
	for _, item := range []struct {
		argument      string
		current, want bool
	}{{"on", false, true}, {"off", true, false}, {"toggle", false, true}, {"", true, false}} {
		got, err := floatingCommandTarget(item.argument, item.current)
		if err != nil || got != item.want {
			t.Error("悬浮按钮命令不正确")
		}
	}
	if _, err := floatingCommandTarget("invalid", false); err == nil {
		t.Error("未知命令应解释用法")
	}
	config, err := parseConfig(`{"floating_widget":{"enabled":true,"positioned":true,"x":-1000,"y":200}}`)
	if err != nil || !config.FloatingWidget.Enabled || config.FloatingWidget.X != -1000 {
		t.Fatalf("位置与启用状态应保存：%+v %v", config, err)
	}
	if !syncLocalKeys["floating_widget"] {
		t.Error("屏幕位置不能同步到其他电脑")
	}
}
