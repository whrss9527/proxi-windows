//go:build windows

package main

import (
	"fmt"
	"image/color"
	"syscall"
	"unicode"
	"unsafe"
)

var procGetMenuItemInfoW = user32.NewProc("GetMenuItemInfoW")

// 自绘菜单需要显式处理字母定位；方向键、Enter 和 Esc 仍交给系统。
func (r *menuHeaderRenderer) menuChar(menu uintptr, character rune) uintptr {
	count, _, _ := user32.NewProc("GetMenuItemCount").Call(menu)
	if count > 10000 {
		return 0
	}
	var matches []uintptr
	selected := ^uintptr(0)
	for position := uintptr(0); position < count; position++ {
		info := menuItemInfoW{mask: miimData | 1}
		info.size = uint32(unsafe.Sizeof(info))
		if ok, _, _ := procGetMenuItemInfoW.Call(menu, position, 1, uintptr(unsafe.Pointer(&info))); ok == 0 {
			continue
		}
		if info.state&0x80 != 0 {
			selected = position
		}
		entry, ok := r.entries[info.itemData]
		if !ok || entry.kind != "" || entry.disabled || info.state&3 != 0 {
			continue
		}
		for _, first := range syscall.UTF16ToString(entry.text) {
			if unicode.ToLower(first) == unicode.ToLower(character) {
				matches = append(matches, position)
			}
			break
		}
	}
	if len(matches) == 1 {
		return matches[0] | 2<<16
	}
	if len(matches) > 1 {
		for _, position := range matches {
			if selected == ^uintptr(0) || position > selected {
				return position | 3<<16
			}
		}
		return matches[0] | 3<<16
	}
	return 0
}

type popupMenuInfo struct {
	size, mask, style, maxHeight uint32
	background                   uintptr
	helpID                       uint32
	data                         uintptr
}

func (r *menuHeaderRenderer) styleMenu(menu uintptr) {
	palette := headerPalette("brand", "", r.dark, r.highContrast, r.systemBackground, r.systemText)
	if r.menuBrush == 0 {
		r.menuBrush, _, _ = procCreateSolidBrush.Call(headerColorRef(palette.background))
	}
	info := popupMenuInfo{mask: 0x80000012, style: 0x80000000, background: r.menuBrush}
	info.size = uint32(unsafe.Sizeof(info))
	user32.NewProc("SetMenuInfo").Call(menu, uintptr(unsafe.Pointer(&info)))
}

func (r *menuHeaderRenderer) textWidth(dc uintptr, text []uint16) int {
	if len(text) < 2 {
		return 0
	}
	rect := menuRect{}
	procDrawTextW.Call(dc, uintptr(unsafe.Pointer(&text[0])), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), dtCalcRect|dtSingleLine|dtNoPrefix)
	return int(rect.right)
}

func (r *menuHeaderRenderer) measureAction(item *measureItemStruct, entry menuHeaderEntry) bool {
	dc, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, dc)
	font := r.font
	if entry.bold || entry.checked && entry.radio {
		font = r.boldFont
	}
	previous, _, _ := procSelectObject.Call(dc, font)
	defer procSelectObject.Call(dc, previous)
	width := r.textWidth(dc, entry.text) + r.textWidth(dc, entry.shortcut) + r.dp(96)
	item.width = uint32(min(max(width, r.dp(280)), r.maxWidth))
	bounds := menuRect{}
	procDrawTextW.Call(dc, uintptr(unsafe.Pointer(&entry.text[0])), ^uintptr(0), uintptr(unsafe.Pointer(&bounds)), dtCalcRect|dtSingleLine|dtNoPrefix)
	item.height = uint32(max(r.dp(32), int(bounds.bottom)+r.dp(12)))
	if entry.kind == "section" {
		item.height = uint32(max(r.dp(26), int(bounds.bottom)+r.dp(8)))
	}
	return true
}

func (r *menuHeaderRenderer) rounded(dc uintptr, rect menuRect, paint color.NRGBA, radius int) {
	brush, _, _ := procCreateSolidBrush.Call(headerColorRef(paint))
	saved, _, _ := procSaveDC.Call(dc)
	procSelectObject.Call(dc, brush)
	pen, _, _ := procGetStockObject.Call(8) // NULL_PEN
	procSelectObject.Call(dc, pen)
	procRoundRect.Call(dc, uintptr(rect.left), uintptr(rect.top), uintptr(rect.right), uintptr(rect.bottom), uintptr(radius), uintptr(radius))
	procRestoreDC.Call(dc, saved)
	procDeleteObject.Call(brush)
}

func (r *menuHeaderRenderer) glyph(dc uintptr, name string, rect menuRect, paint color.NRGBA) {
	if name == "" {
		return
	}
	size := int(rect.right - rect.left)
	key := fmt.Sprintf("%s:%d:%d:%d:%d", name, size, paint.R, paint.G, paint.B)
	if r.glyphs == nil {
		r.glyphs = make(map[string]uintptr)
	}
	icon := r.glyphs[key]
	if icon == 0 {
		icon, _ = createIcon(renderMenuGlyph(size, name, paint))
		r.glyphs[key] = icon
	}
	if icon != 0 {
		procDrawIconEx.Call(dc, uintptr(rect.left), uintptr(rect.top), icon, uintptr(size), uintptr(size), 0, 0, 3)
	}
}

func (r *menuHeaderRenderer) drawAction(item *drawItemStruct, entry menuHeaderEntry) bool {
	saved, _, _ := procSaveDC.Call(item.dc)
	defer procRestoreDC.Call(item.dc, saved)
	p := headerPalette("brand", "", r.dark, r.highContrast, r.systemBackground, r.systemText)
	fillMenuRect(item.dc, item.rect, p.background)
	paint := p.text
	if entry.disabled && !r.highContrast {
		paint = mixHeaderColor(p.text, p.background, .52)
	}
	if entry.kind == "section" {
		if !r.highContrast {
			paint = mixHeaderColor(p.text, p.background, .32)
		}
		box := item.rect
		box.left += int32(r.dp(14))
		box.top += int32(r.dp(4))
		box.right -= int32(r.dp(12))
		procSelectObject.Call(item.dc, r.font)
		procSetBkMode.Call(item.dc, 1)
		procSetTextColor.Call(item.dc, headerColorRef(paint))
		procDrawTextW.Call(item.dc, uintptr(unsafe.Pointer(&entry.text[0])), ^uintptr(0), uintptr(unsafe.Pointer(&box)), dtSingleLine|dtVCenter|dtNoPrefix|dtEndEllipsis)
		return true
	}
	card := item.rect
	card.left += int32(r.dp(6))
	card.right -= int32(r.dp(6))
	card.top += int32(r.dp(2))
	card.bottom -= int32(r.dp(2))
	selected := item.state&1 != 0 && !entry.disabled
	if entry.checked && entry.radio && !r.highContrast {
		active := headerPalette("on", entry.accent, r.dark, false, p.background, p.text)
		r.rounded(item.dc, card, active.card, r.dp(10))
		paint = active.text
	}
	if selected {
		hover := mixHeaderColor(p.background, p.text, .09)
		if r.highContrast {
			hover = menuSystemColor(13)
			paint = menuSystemColor(14)
		}
		r.rounded(item.dc, card, hover, r.dp(10))
	}
	iconRect := menuRect{left: item.rect.left + int32(r.dp(14)), top: item.rect.top + (item.rect.bottom-item.rect.top-int32(r.dp(16)))/2}
	iconRect.right, iconRect.bottom = iconRect.left+int32(r.dp(16)), iconRect.top+int32(r.dp(16))
	iconPaint := paint
	if !r.highContrast && !entry.disabled {
		iconPaint = mixHeaderColor(paint, p.background, .15)
	}
	if entry.icon == "profile" {
		profile := headerPalette("on", entry.accent, r.dark, r.highContrast, p.background, p.text)
		r.glyph(item.dc, "profile", iconRect, profile.accent)
	} else {
		r.glyph(item.dc, entry.icon, iconRect, iconPaint)
	}
	box := item.rect
	box.left += int32(r.dp(42))
	box.right -= int32(r.dp(24))
	font := r.font
	if entry.bold || entry.checked && entry.radio {
		font = r.boldFont
	}
	procSelectObject.Call(item.dc, font)
	procSetBkMode.Call(item.dc, 1)
	if entry.checked {
		mark := iconRect
		mark.left = box.right - int32(r.dp(16))
		mark.right = box.right
		r.glyph(item.dc, "check", mark, paint)
		box.right -= int32(r.dp(24))
	}
	if len(entry.shortcut) > 1 {
		width := min(r.textWidth(item.dc, entry.shortcut), (int(box.right-box.left))/2)
		hint := box
		hint.left = hint.right - int32(width)
		hintPaint := paint
		if !r.highContrast {
			hintPaint = mixHeaderColor(paint, p.background, .25)
		}
		procSetTextColor.Call(item.dc, headerColorRef(hintPaint))
		procDrawTextW.Call(item.dc, uintptr(unsafe.Pointer(&entry.shortcut[0])), ^uintptr(0), uintptr(unsafe.Pointer(&hint)), dtSingleLine|dtVCenter|dtNoPrefix|dtEndEllipsis|2)
		box.right = hint.left - int32(r.dp(16))
	}
	procSetTextColor.Call(item.dc, headerColorRef(paint))
	procDrawTextW.Call(item.dc, uintptr(unsafe.Pointer(&entry.text[0])), ^uintptr(0), uintptr(unsafe.Pointer(&box)), dtSingleLine|dtVCenter|dtNoPrefix|dtEndEllipsis)
	return true
}

// 常用项直接显示，网络功能和低频操作各归一组；保留原来的命令编号及子菜单。
func organizeTrayMenu(source []MenuItem) []MenuItem {
	var top, profiles, tools, network, more, settings, exit []MenuItem
	for index, item := range source {
		if item.Separator {
			continue
		}
		if item.Header != "" || index < 3 && (item.Id == menuToggle || item.Id == menuSettings) {
			if item.Id == menuToggle {
				item.Icon = "power"
			}
			top = append(top, item)
			continue
		}
		switch {
		case item.Id >= menuProfileBase && item.Id < menuChoiceBase || item.Icon == "layers":
			profiles = append(profiles, item)
		case item.Id == menuTest:
			item.Icon = "test"
			tools = append(tools, item)
		case item.Id == menuDiagnose:
			item.Icon = "search"
			tools = append(tools, item)
		case item.Id == menuEditConfig:
			item.Icon = "settings"
			tools = append(tools, item)
		case item.Id == menuTun || item.Id == menuShare || item.Id == menuShareCopy || item.Id == menuAutoSwitch:
			item.Icon = "network"
			network = append(network, item)
		case item.Id == menuFloating:
			item.Icon = "speed"
			tools = append(tools, item)
		case item.Id == menuSettings:
			item.Icon = "settings"
			settings = append(settings, item)
		case item.Id == menuExit:
			item.Icon = "exit"
			exit = append(exit, item)
		default:
			if item.Icon == "" {
				item.Icon = "settings"
			}
			if item.Id == menuUpdate {
				item.Icon = "refresh"
			}
			more = append(more, item)
		}
	}
	result := top
	if len(profiles) > 0 {
		result = append(result, MenuItem{Text: "代理配置", Header: "section", Disabled: true})
		result = append(result, profiles...)
	}
	if len(tools) > 0 || len(network) > 0 {
		result = append(result, MenuItem{Text: "快捷操作", Header: "section", Disabled: true})
		result = append(result, tools...)
		if len(network) > 0 {
			result = append(result, MenuItem{Text: "网络与共享", Icon: "network", Children: network})
		}
	}
	result = append(result, MenuItem{Separator: true})
	result = append(result, settings...)
	if len(more) > 0 {
		result = append(result, MenuItem{Text: "更多", Icon: "more", Children: more})
	}
	result = append(result, MenuItem{Separator: true})
	return append(result, exit...)
}
