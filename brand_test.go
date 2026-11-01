package main

import (
	"bytes"
	"testing"
)

func TestTrayBrandStates(t *testing.T) {
	for _, size := range []int{16, 20, 24, 32, 48, 64} {
		base := renderBrandIcon(size)
		off := renderTrayIcon(size, "brand", iconStateOff, "#2563eb")
		for _, state := range []string{iconStateOn, iconStateOff, iconStateWarn, iconStateExternal, iconStateError} {
			picture := renderTrayIcon(size, "brand", state, "#2563eb")
			if picture.Bounds() != base.Bounds() || picture.NRGBAAt(0, 0).A != 0 {
				t.Fatalf("%dpx 图标边界或透明度不正确", size)
			}
			// 状态只改变角标，应用标识的上半部始终不变。
			for y := 0; y < size/2; y++ {
				for x := 0; x < size; x++ {
					if picture.NRGBAAt(x, y) != base.NRGBAAt(x, y) {
						t.Fatal("状态改变了固定的应用标识")
					}
				}
			}
			if state != iconStateOff && bytes.Equal(picture.Pix, off.Pix) {
				t.Fatal("开启、警告或错误状态与关闭状态无法区分")
			}
		}
		legacy := renderToggleIcon(size, trayIconStyle(iconStateOn, "#2563eb"))
		if !bytes.Equal(legacy.Pix, renderTrayIcon(size, "toggle", iconStateOn, "#2563eb").Pix) {
			t.Fatal("开关样式没有保留原有的配置颜色与开关方向")
		}
	}
}
