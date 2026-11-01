//go:build windows

package main

import (
	"fmt"
	"log/slog"
	"syscall"
	"unsafe"
)

const floatingWindowClass = "ProxiFloatingWidget"

var (
	procBeginPaint          = user32.NewProc("BeginPaint")
	procEndPaint            = user32.NewProc("EndPaint")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procInvalidateRect      = user32.NewProc("InvalidateRect")
	procSetWindowTextW      = user32.NewProc("SetWindowTextW")
	procSetCapture          = user32.NewProc("SetCapture")
	procReleaseCapture      = user32.NewProc("ReleaseCapture")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procCreateRoundRectRgn  = gdi32.NewProc("CreateRoundRectRgn")
	procSetWindowRgn        = user32.NewProc("SetWindowRgn")
	procCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC            = gdi32.NewProc("DeleteDC")
	procBitBlt              = gdi32.NewProc("BitBlt")
	procRoundRect           = gdi32.NewProc("RoundRect")
	procCreatePen           = gdi32.NewProc("CreatePen")
	floatingWindows         = map[uintptr]*floatingWindow{}
	floatingClassRegistered bool
	floatingWindowProcedure uintptr
)

type floatingPaintStruct struct {
	dc                   uintptr
	erase                int32
	rect                 menuRect
	restore, incremental int32
	reserved             [32]byte
}
type floatingWindow struct {
	app              *App
	window           uintptr
	renderer         *menuHeaderRenderer
	icon             uintptr
	iconKey          string
	profile          string
	speed            SpeedInfo
	config           FloatingWidgetConfig
	pressed, dragged bool
	start            point
	origin           menuRect
}

func (app *App) setFloating(enabled bool) error {
	config := app.engine.Config()
	if config == nil {
		return errNoConfig
	}
	updated := config.Clone()
	updated.FloatingWidget.Enabled = enabled
	if err := app.engine.SaveConfig(updated); err != nil {
		return err
	}
	app.applyUiConfig()
	app.refresh()
	return nil
}

func (app *App) syncFloating() {
	config := app.engine.Config()
	if config == nil || !config.FloatingWidget.Enabled {
		if app.floating != nil {
			app.floating.close()
			app.floating = nil
		}
		return
	}
	if app.floating == nil {
		widget, err := newFloatingWindow(app)
		if err != nil {
			slog.Warn("创建悬浮按钮失败", "err", err)
			return
		}
		app.floating = widget
	}
	if app.floating.config != config.FloatingWidget {
		app.floating.config = config.FloatingWidget
		app.floating.place()
	}
	app.floating.refresh(app.engine.Status())
}

// 窗口只在用户单击时打开设置；工具窗口和不激活样式使它不占任务栏、不打断输入。
func newFloatingWindow(app *App) (*floatingWindow, error) {
	if !floatingClassRegistered {
		if floatingWindowProcedure == 0 {
			floatingWindowProcedure = syscall.NewCallback(dispatchFloatingWindow)
		}
		cursor, _, _ := procLoadCursorW.Call(0, 32649)
		class := wndClassExW{style: 0x20000, windowProc: floatingWindowProcedure, instance: moduleHandle(), className: utf16Pointer(floatingWindowClass), cursor: cursor}
		class.size = uint32(unsafe.Sizeof(class))
		if atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
			return nil, fmt.Errorf("注册悬浮窗口：%v", err)
		}
		floatingClassRegistered = true
	}
	widget := &floatingWindow{app: app, renderer: menuHeaderRendererAtDpi(systemDpi())}
	window, _, err := procCreateWindowExW.Call(0x08000088, uintptr(unsafe.Pointer(utf16Pointer(floatingWindowClass))), uintptr(unsafe.Pointer(utf16Pointer(appName+" 悬浮网速"))), 0x80000000, 0, 0, uintptr(widget.renderer.dp(164)), uintptr(widget.renderer.dp(44)), 0, 0, moduleHandle(), 0)
	if window == 0 {
		widget.renderer.close()
		return nil, fmt.Errorf("创建悬浮窗口：%v", err)
	}
	widget.window = window
	floatingWindows[window] = widget
	return widget, nil
}

func floatingWorkArea(rect menuRect) floatingArea {
	monitor, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&rect)), 2)
	info := menuMonitorInfo{}
	info.size = uint32(unsafe.Sizeof(info))
	if result, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); result != 0 {
		return floatingArea{int(info.work.left), int(info.work.top), int(info.work.right), int(info.work.bottom)}
	}
	return floatingArea{0, 0, systemMetric(0), systemMetric(1)}
}

func (widget *floatingWindow) place() {
	if widget.window == 0 || widget.pressed {
		return
	}
	anchor := menuRect{left: int32(widget.config.X), top: int32(widget.config.Y)}
	if !widget.config.Positioned {
		cursor := cursorPosition()
		anchor.left, anchor.top = cursor.x, cursor.y
	}
	anchor.right, anchor.bottom = anchor.left+1, anchor.top+1
	geometry := floatingLayout(widget.config, floatingWorkArea(anchor), widget.renderer.dpi)
	procSetWindowPos.Call(widget.window, ^uintptr(0), uintptr(geometry.x), uintptr(geometry.y), uintptr(geometry.width), uintptr(geometry.height), 0x0050)
	actual := menuRect{}
	procGetWindowRect.Call(widget.window, uintptr(unsafe.Pointer(&actual)))
	if actual.right > actual.left && actual.bottom > actual.top {
		geometry.width, geometry.height = int(actual.right-actual.left), int(actual.bottom-actual.top)
	}
	region, _, _ := procCreateRoundRectRgn.Call(0, 0, uintptr(geometry.width+1), uintptr(geometry.height+1), uintptr(geometry.height), uintptr(geometry.height))
	if region != 0 {
		if ok, _, _ := procSetWindowRgn.Call(widget.window, region, 1); ok == 0 {
			procDeleteObject.Call(region)
		}
	}
}

func (widget *floatingWindow) refresh(status Status) {
	state, paint := widget.app.iconStyleFor(status)
	key := fmt.Sprintf("%s|%s|%d", state, paint, widget.renderer.dpi)
	if key != widget.iconKey {
		if widget.icon != 0 {
			procDestroyIcon.Call(widget.icon)
		}
		widget.icon, _ = createIcon(renderTrayIcon(widget.renderer.dp(28), "brand", state, paint))
		widget.iconKey = key
	}
	widget.profile = "代理已关闭"
	if status.State == statusOn && status.Profile != nil {
		widget.profile = "正在使用「" + status.Profile.Name + "」"
	}
	if state == iconStateWarn || state == iconStateError || state == iconStateExternal {
		widget.profile = "请检查代理状态"
	}
	if widget.app.subscriptionService != nil {
		widget.speed = widget.app.FloatingSpeed()
	}
	speedText := widget.speed.Text()
	if speedText == "" {
		speedText = "网速采样中"
	}
	name := appName + " · " + widget.profile + " · " + speedText + " · 单击设置，拖动移动，右键菜单"
	procSetWindowTextW.Call(widget.window, uintptr(unsafe.Pointer(utf16Pointer(name))))
	procInvalidateRect.Call(widget.window, 0, 0)
}

func (widget *floatingWindow) restyle() {
	if widget.window == 0 {
		return
	}
	dpi := systemDpi()
	if procGetDpiForWindow.Find() == nil {
		if value, _, _ := procGetDpiForWindow.Call(widget.window); value != 0 {
			dpi = int(value)
		}
	}
	widget.renderer.close()
	widget.renderer = menuHeaderRendererAtDpi(dpi)
	widget.iconKey = ""
	widget.place()
	if widget.app != nil {
		widget.refresh(widget.app.engine.Status())
	}
}

func (widget *floatingWindow) close() {
	if widget.window != 0 {
		window := widget.window
		widget.window = 0
		procDestroyWindow.Call(window)
		delete(floatingWindows, window)
	}
	if widget.icon != 0 {
		procDestroyIcon.Call(widget.icon)
		widget.icon = 0
	}
	if widget.renderer != nil {
		widget.renderer.close()
		widget.renderer = nil
	}
}

func (widget *floatingWindow) paint(dc uintptr, rect menuRect) {
	r := widget.renderer
	palette := headerPalette("brand", "", r.dark, r.highContrast, r.systemBackground, r.systemText)
	fillMenuRect(dc, rect, palette.background)
	brush, _, _ := procCreateSolidBrush.Call(headerColorRef(palette.background))
	pen, _, _ := procCreatePen.Call(0, uintptr(max(1, r.dp(1))), headerColorRef(mixHeaderColor(palette.background, palette.text, .18)))
	saved, _, _ := procSaveDC.Call(dc)
	procSelectObject.Call(dc, brush)
	procSelectObject.Call(dc, pen)
	procRoundRect.Call(dc, 0, 0, uintptr(rect.right), uintptr(rect.bottom), uintptr(rect.bottom), uintptr(rect.bottom))
	procRestoreDC.Call(dc, saved)
	procDeleteObject.Call(brush)
	procDeleteObject.Call(pen)
	if widget.icon != 0 {
		procDrawIconEx.Call(dc, uintptr(r.dp(9)), uintptr((int(rect.bottom)-r.dp(28))/2), widget.icon, uintptr(r.dp(28)), uintptr(r.dp(28)), 0, 0, 3)
	}
	divider := menuRect{left: int32(r.dp(44)), top: int32(r.dp(10)), right: int32(r.dp(45)), bottom: rect.bottom - int32(r.dp(10))}
	fillMenuRect(dc, divider, mixHeaderColor(palette.background, palette.text, .14))
	down, up := "↓ 采样中…", "↑ —"
	if widget.speed.Ready && widget.speed.Mode == speedSystem {
		down, up = "↓ "+formatSpeed(widget.speed.Download), "↑ "+formatSpeed(widget.speed.Upload)
	}
	procSelectObject.Call(dc, r.font)
	procSetBkMode.Call(dc, 1)
	for index, text := range []string{down, up} {
		box := menuRect{left: int32(r.dp(54)), top: int32(r.dp(2 + index*20)), right: rect.right - int32(r.dp(10)), bottom: int32(r.dp(22 + index*20))}
		paint := palette.text
		if index == 1 && !r.highContrast {
			paint = mixHeaderColor(palette.text, palette.background, .18)
		}
		procSetTextColor.Call(dc, headerColorRef(paint))
		procDrawTextW.Call(dc, uintptr(unsafe.Pointer(utf16Pointer(text))), ^uintptr(0), uintptr(unsafe.Pointer(&box)), dtSingleLine|dtVCenter|dtNoPrefix|dtEndEllipsis)
	}
}

func (widget *floatingWindow) procedure(window, message, wParam, lParam uintptr) uintptr {
	switch uint32(message) {
	case 0x21:
		return 3 // 不激活窗口，但仍接收点击。
	case 0x14:
		return 1 // 背景在双缓冲绘制中完整覆盖。
	case 0x0f:
		var paint floatingPaintStruct
		dc, _, _ := procBeginPaint.Call(window, uintptr(unsafe.Pointer(&paint)))
		rect := menuRect{}
		procGetClientRect.Call(window, uintptr(unsafe.Pointer(&rect)))
		if rect.right > 0 && rect.bottom > 0 {
			bitmap, _, err := createDibSection(int(rect.right), int(rect.bottom))
			if err == nil {
				buffer, _, _ := procCreateCompatibleDC.Call(dc)
				previous, _, _ := procSelectObject.Call(buffer, bitmap)
				widget.paint(buffer, rect)
				procBitBlt.Call(dc, 0, 0, uintptr(rect.right), uintptr(rect.bottom), buffer, 0, 0, 0x00cc0020)
				procSelectObject.Call(buffer, previous)
				procDeleteDC.Call(buffer)
				procDeleteObject.Call(bitmap)
			}
		}
		procEndPaint.Call(window, uintptr(unsafe.Pointer(&paint)))
		return 0
	case 0x201:
		widget.pressed = true
		widget.dragged = false
		widget.start = cursorPosition()
		procGetWindowRect.Call(window, uintptr(unsafe.Pointer(&widget.origin)))
		procSetCapture.Call(window)
		return 0
	case 0x200:
		if widget.pressed {
			cursor := cursorPosition()
			dx, dy := cursor.x-widget.start.x, cursor.y-widget.start.y
			threshold := int32(widget.renderer.dp(4))
			if dx > threshold || dx < -threshold || dy > threshold || dy < -threshold {
				widget.dragged = true
			}
			if widget.dragged {
				procSetWindowPos.Call(window, 0, uintptr(widget.origin.left+dx), uintptr(widget.origin.top+dy), 0, 0, 0x0015)
			}
		}
		return 0
	case 0x202:
		if widget.pressed {
			dragged := widget.dragged
			widget.pressed = false
			procReleaseCapture.Call()
			if dragged {
				rect := menuRect{}
				procGetWindowRect.Call(window, uintptr(unsafe.Pointer(&rect)))
				config := widget.app.engine.Config().Clone()
				config.FloatingWidget.Positioned = true
				config.FloatingWidget.X = int(rect.left)
				config.FloatingWidget.Y = int(rect.top)
				geometry := floatingLayout(config.FloatingWidget, floatingWorkArea(rect), widget.renderer.dpi)
				config.FloatingWidget.X, config.FloatingWidget.Y = geometry.x, geometry.y
				if err := widget.app.engine.SaveConfig(config); err == nil {
					widget.app.applyUiConfig()
				}
			} else {
				widget.app.openSettings()
			}
		}
		return 0
	case 0x215:
		widget.pressed = false
		return 0
	case wmContextMenu:
		if widget.app != nil {
			widget.app.onTrayMenu(cursorPosition())
		}
		return 0
	case wmDpiChanged, wmDisplayChange:
		widget.restyle()
		return 0
	case wmClose:
		if widget.app != nil {
			_ = widget.app.setFloating(false)
		} else {
			widget.close()
		}
		return 0
	case 0x82:
		delete(floatingWindows, window)
	}
	result, _, _ := procDefWindowProcW.Call(window, message, wParam, lParam)
	return result
}

func dispatchFloatingWindow(window, message, wParam, lParam uintptr) uintptr {
	if widget := floatingWindows[window]; widget != nil {
		return widget.procedure(window, message, wParam, lParam)
	}
	result, _, _ := procDefWindowProcW.Call(window, message, wParam, lParam)
	return result
}
