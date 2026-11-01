//go:build windows

package main

import (
	"runtime"
	"testing"
)

func TestSettingsWindowIconBinding(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	widget, err := newFloatingWindow(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer widget.close()
	icons := settingsWindowIcons{}
	defer icons.close()
	for _, dpi := range []int{96, 144, 192, 288} {
		icons.apply(widget.window, dpi)
		for kind, size := range []int{16 * dpi / 96, 32 * dpi / 96} {
			current, ok := settingsIconMessage(widget.window, wmGetIcon, uintptr(kind), uintptr(dpi))
			if !ok || current == 0 || current != icons.cache[size] {
				t.Fatalf("%d DPI 的窗口没有绑定原尺寸图标：kind=%d", dpi, kind)
			}
		}
		// 浏览器重新设置 favicon 后，下次刷新应补回清晰的原尺寸图标。
		settingsIconMessage(widget.window, wmSetIcon, 1, 0)
		icons.apply(widget.window, dpi)
		if current, _ := settingsIconMessage(widget.window, wmGetIcon, 1, 0); current != icons.cache[32*dpi/96] {
			t.Fatal("浏览器覆盖后没有恢复图标")
		}
	}
	icons.close()
	icons.close()
	for kind := uintptr(0); kind < 2; kind++ {
		if current, _ := settingsIconMessage(widget.window, wmGetIcon, kind, 0); current != 0 {
			t.Error("释放图标前必须解除窗口引用")
		}
	}
	if len(icons.cache) != 0 || len(icons.windows) != 0 {
		t.Error("关闭后不能留下缓存或窗口引用")
	}
}

func TestSettingsWindowIconScope(t *testing.T) {
	if !settingsIconWindowMatches(settingsWindowTitle, "Chrome_WidgetWin_1") {
		t.Error("应识别独立设置窗口")
	}
	for _, item := range [][2]string{{settingsWindowTitle + " - Microsoft Edge", "Chrome_WidgetWin_1"}, {"其他应用", "Chrome_WidgetWin_1"}, {settingsWindowTitle, "其他窗口"}} {
		if settingsIconWindowMatches(item[0], item[1]) {
			t.Error("不能改动普通浏览器或其他应用的图标")
		}
	}
}
