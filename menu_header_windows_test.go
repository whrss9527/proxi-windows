//go:build windows

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

var (
	procGetMenuItemInfoW = user32.NewProc("GetMenuItemInfoW")
)

func TestMenuHeaderDisabledButKeepsFullText(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r := menuHeaderRendererAtDpi(96)
	defer r.close()
	var texts []*uint16
	menu := buildMenu([]MenuItem{
		{Text: "Proxi", Header: "brand"},
		{Id: 1, Text: "代理已开启：公司 && 内网", Header: "on", Accent: "#16a34a"},
		{Id: 2, Text: "关闭代理"},
	}, &texts, r)
	if menu == 0 {
		t.Fatal("创建原生菜单失败")
	}
	defer procDestroyMenu.Call(menu)
	for position := uintptr(0); position < 3; position++ {
		buffer := make([]uint16, 128)
		info := menuItemInfoW{mask: miimFType | miimData | miimString | 1, typeData: &buffer[0], length: uint32(len(buffer))}
		info.size = uint32(unsafe.Sizeof(info))
		result, _, err := procGetMenuItemInfoW.Call(menu, position, 1, uintptr(unsafe.Pointer(&info)))
		if result == 0 {
			t.Fatalf("读取原生菜单失败：%v", err)
		}
		if position < 2 {
			if info.state&3 == 0 || info.kind&mftOwnerDraw == 0 || info.itemData == 0 {
				t.Fatalf("顶部信息应不可操作并保留自绘数据：%+v", info)
			}
			if got := syscall.UTF16ToString(buffer); got == "" || position == 1 && got != "代理已开启：公司 & 内网" {
				t.Errorf("原生菜单应保留可读取的完整文字，得到 %q", got)
			}
		} else if info.state&3 != 0 || info.kind&mftOwnerDraw != 0 {
			t.Error("操作项应继续使用原生菜单，并允许点击和键盘选择")
		}
	}
}

func TestMenuHeaderNativeRendering(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, dpi := range []int{96, 144, 192} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("dpi%d-dark%v", dpi, dark), func(t *testing.T) {
				r := menuHeaderRendererAtDpi(dpi)
				defer r.close()
				var actualFont logFontW
				gdi32.NewProc("GetObjectW").Call(r.font, unsafe.Sizeof(actualFont), uintptr(unsafe.Pointer(&actualFont)))
				t.Logf("原生菜单字体：%s，高度 %d", syscall.UTF16ToString(actualFont.faceName[:]), actualFont.height)
				r.dark, r.highContrast = dark, false
				r.entries[1] = menuHeaderEntry{text: syscall.StringToUTF16("Proxi"), kind: "brand"}
				r.entries[2] = menuHeaderEntry{text: syscall.StringToUTF16("代理已开启：公司内网"), kind: "on", accent: "#16a34a"}
				brand := measureItemStruct{controlType: odtMenu, itemData: 1}
				status := measureItemStruct{controlType: odtMenu, itemData: 2}
				if !r.measure(&brand) || !r.measure(&status) || status.height <= brand.height {
					t.Fatal("顶部状态应有合适的独立高度")
				}
				width, height := int(max(brand.width, status.width)), int(brand.height+status.height)
				bitmap, pixels, err := createDibSection(width, height)
				if err != nil {
					t.Fatal(err)
				}
				defer procDeleteObject.Call(bitmap)
				dc, _, _ := procCreateCompatibleDC.Call(0)
				if dc == 0 {
					t.Fatal("创建绘图上下文失败")
				}
				defer procDeleteDC.Call(dc)
				previous, _, _ := procSelectObject.Call(dc, bitmap)
				defer procSelectObject.Call(dc, previous)
				// 即使系统传来禁用、变灰和选中标志，信息区也保持清晰，不显示可点击的悬停效果。
				first := drawItemStruct{controlType: odtMenu, dc: dc, rect: menuRect{right: int32(width), bottom: int32(brand.height)}, itemData: 1, state: 7}
				second := drawItemStruct{controlType: odtMenu, dc: dc, rect: menuRect{top: int32(brand.height), right: int32(width), bottom: int32(height)}, itemData: 2, state: 7}
				if !r.draw(&first) || !r.draw(&second) {
					t.Fatal("绘制状态区失败")
				}
				gdi32.NewProc("GdiFlush").Call()
				picture := image.NewNRGBA(image.Rect(0, 0, width, height))
				for offset := 0; offset < len(pixels); offset += 4 {
					copy(picture.Pix[offset:offset+4], []byte{pixels[offset+2], pixels[offset+1], pixels[offset], 255})
				}
				palette := headerPalette("on", "#16a34a", dark, false, color.NRGBA{}, color.NRGBA{})
				if paint := picture.NRGBAAt(r.dp(7), int(brand.height)+r.dp(10)); paint != palette.accent {
					t.Errorf("禁用标志不应将状态强调色变灰：得到 %+v，期望 %+v", paint, palette.accent)
				}
				// 很长的名字限制菜单宽度，完整名字仍存在菜单信息和设置页面中。
				r.entries[3] = menuHeaderEntry{text: syscall.StringToUTF16(strings.Repeat("公司内网", 100)), kind: "on"}
				long := measureItemStruct{controlType: odtMenu, itemData: 3}
				r.measure(&long)
				if long.width > uint32(r.dp(480)) {
					t.Error("长配置名不应撑破屏幕")
				}
				r.maxWidth = r.dp(220)
				r.measure(&long)
				if long.width > uint32(r.maxWidth) {
					t.Error("较小显示器也要遵守实际可用宽度")
				}
				if folder := os.Getenv("PROXI_MENU_SNAPSHOTS"); folder != "" {
					if err := os.MkdirAll(folder, 0755); err != nil {
						t.Fatal(err)
					}
					file, err := os.Create(filepath.Join(folder, fmt.Sprintf("menu-header-dark%v-%d.png", dark, dpi)))
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(file, picture)
					file.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}
