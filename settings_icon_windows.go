//go:build windows

package main

import (
	"log/slog"
	"strings"
	"syscall"
	"unsafe"
)

const (
	wmGetIcon = 0x007f
	wmSetIcon = 0x0080
)

// 浏览器会把小尺寸 favicon 放大作为应用窗口图标。直接按窗口 DPI 绘制原始图形，
// 并补回被浏览器刷新覆盖的图标；只处理标题完全匹配的独立设置窗口。
type settingsWindowIcons struct {
	cache   map[int]uintptr
	windows map[uintptr]bool
}

var (
	procIsWindow            = user32.NewProc("IsWindow")
	settingsIconWindows     []uintptr
	enumSettingsIconWindows = syscall.NewCallback(func(window, parameter uintptr) uintptr {
		if visible, _, _ := procIsWindowVisible.Call(window); visible == 0 {
			return 1
		}
		title, class := make([]uint16, 256), make([]uint16, 64)
		procGetWindowTextW.Call(window, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		procGetClassNameW.Call(window, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
		if settingsIconWindowMatches(syscall.UTF16ToString(title), syscall.UTF16ToString(class)) {
			settingsIconWindows = append(settingsIconWindows, window)
		}
		return 1
	})
)

func settingsIconWindowMatches(title, class string) bool {
	return title == settingsWindowTitle && strings.HasPrefix(class, "Chrome_WidgetWin_")
}

func settingsIconMessage(window, message, kind, icon uintptr) (uintptr, bool) {
	var result uintptr
	ok, _, _ := procSendMessageTimeoutW.Call(window, message, kind, icon, smtoAbortIfHung, 50, uintptr(unsafe.Pointer(&result)))
	return result, ok != 0
}

func (icons *settingsWindowIcons) icon(size int) (uintptr, error) {
	if handle := icons.cache[size]; handle != 0 {
		return handle, nil
	}
	handle, err := createIcon(renderBrandIcon(size))
	if err != nil {
		return 0, err
	}
	if icons.cache == nil {
		icons.cache = make(map[int]uintptr)
	}
	icons.cache[size] = handle
	return handle, nil
}

func (icons *settingsWindowIcons) apply(window uintptr, dpi int) {
	for kind, logicalSize := range []int{16, 32} {
		size := max(16, min(256, (logicalSize*max(96, dpi)+48)/96))
		current, ok := settingsIconMessage(window, wmGetIcon, uintptr(kind), uintptr(dpi))
		if !ok {
			continue
		}
		handle, err := icons.icon(size)
		if err != nil {
			slog.Warn("创建设置窗口图标失败", "size", size, "err", err)
			continue
		}
		if current != handle {
			if _, ok = settingsIconMessage(window, wmSetIcon, uintptr(kind), handle); !ok {
				continue
			}
		}
		if icons.windows == nil {
			icons.windows = make(map[uintptr]bool)
		}
		icons.windows[window] = true
	}
}

// 与托盘刷新共用 UI 线程和定时器，不创建额外轮询线程，不激活窗口。
func (icons *settingsWindowIcons) refresh(enabled bool) {
	if !enabled {
		icons.close()
		return
	}
	settingsIconWindows = settingsIconWindows[:0]
	procEnumWindows.Call(enumSettingsIconWindows, 0)
	for _, window := range settingsIconWindows {
		dpi := systemDpi()
		if procGetDpiForWindow.Find() == nil {
			if value, _, _ := procGetDpiForWindow.Call(window); value != 0 {
				dpi = int(value)
			}
		}
		icons.apply(window, dpi)
	}
	for window := range icons.windows {
		if valid, _, _ := procIsWindow.Call(window); valid == 0 {
			delete(icons.windows, window)
		}
	}
}

func (icons *settingsWindowIcons) owns(handle uintptr) bool {
	if handle == 0 {
		return false
	}
	for _, cached := range icons.cache {
		if cached == handle {
			return true
		}
	}
	return false
}

func (icons *settingsWindowIcons) close() {
	for window := range icons.windows {
		if valid, _, _ := procIsWindow.Call(window); valid == 0 {
			delete(icons.windows, window)
			continue
		}
		for kind := uintptr(0); kind < 2; kind++ {
			current, ok := settingsIconMessage(window, wmGetIcon, kind, 0)
			if !ok {
				return // 窗口无响应时保留资源，避免仍被使用的图标句柄失效。
			}
			if icons.owns(current) {
				if _, ok := settingsIconMessage(window, wmSetIcon, kind, 0); !ok {
					return
				}
			}
		}
		delete(icons.windows, window)
	}
	for _, handle := range icons.cache {
		procDestroyIcon.Call(handle)
	}
	icons.cache = nil
}
