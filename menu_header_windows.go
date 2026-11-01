//go:build windows

package main

import (
	"image/color"
	"strings"
	"syscall"
	"unsafe"
)

const (
	wmDrawItem             = 0x002B
	wmMeasureItem          = 0x002C
	odtMenu                = 1
	mftOwnerDraw           = 0x0100
	miimData               = 0x0020
	miimString             = 0x0040
	spiGetNonClientMetrics = 0x0029
	spiGetHighContrast     = 0x0042
	dtSingleLine           = 0x0020
	dtVCenter              = 0x0004
	dtCalcRect             = 0x0400
	dtNoPrefix             = 0x0800
	dtEndEllipsis          = 0x8000
)

var (
	procGetDC                      = user32.NewProc("GetDC")
	procReleaseDC                  = user32.NewProc("ReleaseDC")
	procDrawTextW                  = user32.NewProc("DrawTextW")
	procFillRect                   = user32.NewProc("FillRect")
	procDrawIconEx                 = user32.NewProc("DrawIconEx")
	procGetSysColor                = user32.NewProc("GetSysColor")
	procGetDpiForWindow            = user32.NewProc("GetDpiForWindow")
	procSystemParametersInfoW      = user32.NewProc("SystemParametersInfoW")
	procSystemParametersInfoForDpi = user32.NewProc("SystemParametersInfoForDpi")
	procCreateFontIndirectW        = gdi32.NewProc("CreateFontIndirectW")
	procGetStockObject             = gdi32.NewProc("GetStockObject")
	procSelectObject               = gdi32.NewProc("SelectObject")
	procSaveDC                     = gdi32.NewProc("SaveDC")
	procRestoreDC                  = gdi32.NewProc("RestoreDC")
	procSetBkMode                  = gdi32.NewProc("SetBkMode")
	procSetTextColor               = gdi32.NewProc("SetTextColor")
	procCreateSolidBrush           = gdi32.NewProc("CreateSolidBrush")
	procMonitorFromRect            = user32.NewProc("MonitorFromRect")
	procGetMonitorInfoW            = user32.NewProc("GetMonitorInfoW")
)

type menuRect struct{ left, top, right, bottom int32 }
type menuMonitorInfo struct {
	size          uint32
	monitor, work menuRect
	flags         uint32
}
type logFontW struct {
	height, width, escapement, orientation, weight                                              int32
	italic, underline, strikeOut, charSet, outPrecision, clipPrecision, quality, pitchAndFamily uint8
	faceName                                                                                    [32]uint16
}
type nonClientMetricsW struct {
	size                                                                uint32
	borderWidth, scrollWidth, scrollHeight, captionWidth, captionHeight int32
	captionFont                                                         logFontW
	smallCaptionWidth, smallCaptionHeight                               int32
	smallCaptionFont                                                    logFontW
	menuWidth, menuHeight                                               int32
	menuFont, statusFont, messageFont                                   logFontW
	paddedBorderWidth                                                   int32
}
type highContrastW struct {
	size, flags uint32
	scheme      uintptr
}
type measureItemStruct struct {
	controlType, controlID, itemID, width, height uint32
	itemData                                      uintptr
}
type drawItemStruct struct {
	controlType, controlID, itemID, action, state uint32
	window, dc                                    uintptr
	rect                                          menuRect
	itemData                                      uintptr
}
type menuHeaderEntry struct {
	text         []uint16
	kind, accent string
}
type menuHeaderRenderer struct {
	dpi                          int
	maxWidth                     int
	font, boldFont, icon         uintptr
	ownedFonts                   []uintptr
	entries                      map[uintptr]menuHeaderEntry
	dark, highContrast           bool
	systemBackground, systemText color.NRGBA
}

func newMenuHeaderRenderer(window uintptr) *menuHeaderRenderer {
	dpi := systemDpi()
	if procGetDpiForWindow.Find() == nil && window != 0 {
		if value, _, _ := procGetDpiForWindow.Call(window); value != 0 {
			dpi = int(value)
		}
	}
	return menuHeaderRendererAtDpi(dpi)
}

func menuHeaderRendererAtDpi(dpi int) *menuHeaderRenderer {
	r := &menuHeaderRenderer{dpi: max(96, dpi), entries: map[uintptr]menuHeaderEntry{}}
	r.maxWidth = r.dp(480)
	light, ok := readRegistryDword(hkeyCurrentUser, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "AppsUseLightTheme")
	r.dark = ok && light == 0 && windowsBuild() >= minimumDarkMenuBuild
	hc := highContrastW{}
	hc.size = uint32(unsafe.Sizeof(hc))
	procSystemParametersInfoW.Call(spiGetHighContrast, uintptr(hc.size), uintptr(unsafe.Pointer(&hc)), 0)
	r.highContrast = hc.flags&1 != 0
	r.systemBackground, r.systemText = menuSystemColor(4), menuSystemColor(7)
	metrics := nonClientMetricsW{}
	metrics.size = uint32(unsafe.Sizeof(metrics))
	var loaded uintptr
	if procSystemParametersInfoForDpi.Find() == nil {
		loaded, _, _ = procSystemParametersInfoForDpi.Call(spiGetNonClientMetrics, uintptr(metrics.size), uintptr(unsafe.Pointer(&metrics)), 0, uintptr(r.dpi))
	}
	if loaded == 0 {
		procSystemParametersInfoW.Call(spiGetNonClientMetrics, uintptr(metrics.size), uintptr(unsafe.Pointer(&metrics)), 0)
		metrics.menuFont.height = int32(int(metrics.menuFont.height) * r.dpi / systemDpi())
	}
	if metrics.menuFont.height == 0 {
		metrics.menuFont.height = -int32(r.dp(12))
		copy(metrics.menuFont.faceName[:], syscall.StringToUTF16("Segoe UI"))
	}
	r.font, _, _ = procCreateFontIndirectW.Call(uintptr(unsafe.Pointer(&metrics.menuFont)))
	if r.font != 0 {
		r.ownedFonts = append(r.ownedFonts, r.font)
	} else {
		r.font, _, _ = procGetStockObject.Call(17)
	}
	metrics.menuFont.weight = 700
	r.boldFont, _, _ = procCreateFontIndirectW.Call(uintptr(unsafe.Pointer(&metrics.menuFont)))
	if r.boldFont != 0 {
		r.ownedFonts = append(r.ownedFonts, r.boldFont)
	} else {
		r.boldFont = r.font
	}
	r.icon, _ = createIcon(renderBrandIcon(r.dp(18)))
	return r
}

func (r *menuHeaderRenderer) dp(value int) int { return (value*r.dpi + 48) / 96 }
func (r *menuHeaderRenderer) close() {
	for _, font := range r.ownedFonts {
		procDeleteObject.Call(font)
	}
	if r.icon != 0 {
		procDestroyIcon.Call(r.icon)
	}
}

func (r *menuHeaderRenderer) register(menu, position uintptr, item MenuItem) {
	key := uintptr(len(r.entries) + 1)
	text := syscall.StringToUTF16(strings.NewReplacer("&&", "&", "\x00", " ", "\r", " ", "\n", " ").Replace(item.Text))
	r.entries[key] = menuHeaderEntry{text: text, kind: item.Header, accent: item.Accent}
	// 保留完整文本给系统菜单的辅助功能接口，不把 Go 指针存入原生菜单。
	info := menuItemInfoW{mask: miimFType | miimData | miimString, kind: mftOwnerDraw, itemData: key, typeData: &text[0]}
	info.size = uint32(unsafe.Sizeof(info))
	procSetMenuItemInfoW.Call(menu, position, 1, uintptr(unsafe.Pointer(&info)))
}

func (r *menuHeaderRenderer) measure(item *measureItemStruct) bool {
	if item == nil || item.controlType != odtMenu {
		return false
	}
	entry, ok := r.entries[item.itemData]
	if !ok {
		return false
	}
	dc, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, dc)
	font := r.boldFont
	if entry.kind == "brand" {
		font = r.font
	}
	previous, _, _ := procSelectObject.Call(dc, font)
	defer procSelectObject.Call(dc, previous)
	rect := menuRect{}
	procDrawTextW.Call(dc, uintptr(unsafe.Pointer(&entry.text[0])), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), dtCalcRect|dtSingleLine|dtNoPrefix)
	item.width = uint32(min(max(int(rect.right)+r.dp(56), r.dp(280)), r.maxWidth))
	height := max(r.dp(38), int(rect.bottom)+r.dp(16))
	if entry.kind == "brand" {
		height = max(r.dp(28), int(rect.bottom)+r.dp(8))
	}
	item.height = uint32(height)
	return true
}

// 菜单靠近哪个显示器，就按那个显示器的工作区限制长配置名的宽度。
func (r *menuHeaderRenderer) fitMonitor(anchor point) {
	rect := menuRect{anchor.x, anchor.y, anchor.x + 1, anchor.y + 1}
	monitor, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&rect)), 2)
	info := menuMonitorInfo{}
	info.size = uint32(unsafe.Sizeof(info))
	if result, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); result != 0 {
		available := int(info.work.right-info.work.left) - r.dp(32)
		if available > 0 {
			r.maxWidth = min(r.maxWidth, available)
		}
	}
}

func (r *menuHeaderRenderer) draw(item *drawItemStruct) bool {
	if item == nil || item.controlType != odtMenu || item.dc == 0 {
		return false
	}
	entry, ok := r.entries[item.itemData]
	if !ok {
		return false
	}
	state, _, _ := procSaveDC.Call(item.dc)
	defer procRestoreDC.Call(item.dc, state)
	palette := headerPalette(entry.kind, entry.accent, r.dark, r.highContrast, r.systemBackground, r.systemText)
	fillMenuRect(item.dc, item.rect, palette.background)
	textRect := item.rect
	textRect.left += int32(r.dp(34))
	textRect.right -= int32(r.dp(16))
	font := r.boldFont
	if entry.kind == "brand" {
		font = r.font
		if r.icon != 0 {
			size := r.dp(18)
			y := item.rect.top + (item.rect.bottom-item.rect.top-int32(size))/2
			procDrawIconEx.Call(item.dc, uintptr(item.rect.left+int32(r.dp(10))), uintptr(y), r.icon, uintptr(size), uintptr(size), 0, 0, 3)
		}
	} else {
		card := item.rect
		card.left += int32(r.dp(6))
		card.right -= int32(r.dp(6))
		card.top += int32(r.dp(2))
		card.bottom -= int32(r.dp(3))
		fillMenuRect(item.dc, card, palette.card)
		stripe := card
		stripe.right = stripe.left + int32(r.dp(3))
		fillMenuRect(item.dc, stripe, palette.accent)
		textRect.left = card.left + int32(r.dp(14))
	}
	procSelectObject.Call(item.dc, font)
	procSetBkMode.Call(item.dc, 1)
	procSetTextColor.Call(item.dc, headerColorRef(palette.text))
	procDrawTextW.Call(item.dc, uintptr(unsafe.Pointer(&entry.text[0])), ^uintptr(0), uintptr(unsafe.Pointer(&textRect)), dtSingleLine|dtVCenter|dtNoPrefix|dtEndEllipsis)
	return true
}

func headerColorRef(c color.NRGBA) uintptr { return uintptr(c.R) | uintptr(c.G)<<8 | uintptr(c.B)<<16 }
func menuSystemColor(index uintptr) color.NRGBA {
	value, _, _ := procGetSysColor.Call(index)
	return color.NRGBA{uint8(value), uint8(value >> 8), uint8(value >> 16), 255}
}
func fillMenuRect(dc uintptr, rect menuRect, paint color.NRGBA) {
	brush, _, _ := procCreateSolidBrush.Call(headerColorRef(paint))
	if brush != 0 {
		procFillRect.Call(dc, uintptr(unsafe.Pointer(&rect)), brush)
		procDeleteObject.Call(brush)
	}
}
