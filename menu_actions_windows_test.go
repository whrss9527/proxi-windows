//go:build windows

package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"unsafe"
)

func menuDesignFixture() []MenuItem {
	return []MenuItem{
		{Text: appName + "\tv" + appVersion, Header: "brand"}, {Id: menuHeader, Text: "代理已开启：公司内网", Header: "on", Accent: "#16a34a"},
		{Id: menuToggle, Text: "关闭代理\tCtrl+Alt+P"},
		{Id: menuProfileBase, Text: "公司内网", Icon: "profile", Accent: "#16a34a", Radio: true, Checked: true},
		{Id: menuProfileBase + 1, Text: "bywave", Icon: "profile", Accent: "#2563eb", Radio: true, Children: []MenuItem{{Id: 1001, Text: "自动选择（延迟最低）", Radio: true, Checked: true}, {Id: 1002, Text: "日本 01\t68 ms", Radio: true}}},
		{Id: menuTest, Text: "测试代理连接"}, {Id: menuDiagnose, Text: "网址诊断…"},
		{Id: menuTun, Text: "TUN 模式", Checked: true}, {Id: menuShare, Text: "局域网共享"},
		{Id: menuFloating, Text: "悬浮网速按钮", Checked: true}, {Id: menuFloatingReset, Text: "重置悬浮位置"},
		{Id: menuSettings, Text: "设置…", Default: true}, {Id: menuAutostart, Text: "开机自动启动", Checked: true},
		{Id: menuUpdate, Text: "检查更新…"}, {Id: menuExit, Text: "退出"},
	}
}

func collectMenuCommands(items []MenuItem, result map[uint32]int) {
	for _, item := range items {
		if item.Id != 0 {
			result[item.Id]++
		}
		collectMenuCommands(item.Children, result)
	}
}

func TestTrayMenuOrganizationPreservesCommands(t *testing.T) {
	source := menuDesignFixture()
	organized := organizeTrayMenu(source)
	before, after := map[uint32]int{}, map[uint32]int{}
	collectMenuCommands(source, before)
	collectMenuCommands(organized, after)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("分组不能丢失或复制命令：%v -> %v", before, after)
	}
	if organized[0].Header != "brand" || organized[1].Header != "on" || organized[2].Id != menuToggle {
		t.Fatal("状态与主操作必须固定在顶部")
	}
	if organized[len(organized)-1].Id != menuExit {
		t.Fatal("退出应在菜单底部")
	}
	for index, item := range organized {
		if item.Separator && (index == 0 || organized[index-1].Separator) {
			t.Error("菜单不能出现多余分隔线")
		}
	}
}

func TestTrayMenuKeyboardLetters(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r := menuHeaderRendererAtDpi(96)
	defer r.close()
	var texts []*uint16
	menu := buildMenu([]MenuItem{{Text: appName, Header: "brand"}, {Id: 1, Text: "bywave", Children: []MenuItem{{Id: 11, Text: "使用这个配置"}}}, {Id: 2, Text: "Backup", Disabled: true}}, &texts, r)
	if menu == 0 {
		t.Fatal("创建菜单失败")
	}
	defer procDestroyMenu.Call(menu)
	if result := r.menuChar(menu, 'B'); result != 1|2<<16 {
		t.Fatalf("字母应定位可用配置，并忽略禁用项：%x", result)
	}
	if result := r.menuChar(menu, 'P'); result != 0 {
		t.Error("不能通过字母激活只读品牌信息")
	}
}

func TestTrayMenuNativeRowsAndRendering(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, dpi := range []int{96, 144, 192} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("dpi%d-dark%v", dpi, dark), func(t *testing.T) {
				r := menuHeaderRendererAtDpi(dpi)
				defer r.close()
				r.dark, r.highContrast = dark, false
				items := organizeTrayMenu(menuDesignFixture())
				var texts []*uint16
				menu := buildMenu(items, &texts, r)
				if menu == 0 {
					t.Fatal("创建菜单失败")
				}
				defer procDestroyMenu.Call(menu)
				r.styleMenu(menu)
				var checkStyle func(uintptr, []MenuItem)
				checkStyle = func(handle uintptr, children []MenuItem) {
					info := popupMenuInfo{mask: 0x12}
					info.size = uint32(unsafe.Sizeof(info))
					if ok, _, _ := user32.NewProc("GetMenuInfo").Call(handle, uintptr(unsafe.Pointer(&info))); ok == 0 || info.style&0x80000000 == 0 || info.background != r.menuBrush {
						t.Fatal("菜单和子菜单必须统一背景并取消原生勾选留白")
					}
					for index, child := range children {
						if len(child.Children) > 0 {
							sub, _, _ := user32.NewProc("GetSubMenu").Call(handle, uintptr(index))
							checkStyle(sub, child.Children)
						}
					}
				}
				checkStyle(menu, items)
				if command, _, _ := user32.NewProc("GetMenuDefaultItem").Call(menu, 0, 0); command != menuSettings {
					t.Fatal("设置应继续是默认操作")
				}
				type row struct {
					key     uintptr
					height  uint32
					id      uint32
					submenu bool
				}
				rows := []row{}
				width, height := 0, 0
				for index, item := range items {
					if item.Separator {
						rows = append(rows, row{height: uint32(r.dp(10))})
						height += r.dp(10)
						continue
					}
					info := menuItemInfoW{mask: miimFType | miimData | 1}
					info.size = uint32(unsafe.Sizeof(info))
					if ok, _, _ := procGetMenuItemInfoW.Call(menu, uintptr(index), 1, uintptr(unsafe.Pointer(&info))); ok == 0 {
						t.Fatal("读取菜单项失败")
					}
					if info.kind&mftOwnerDraw == 0 || info.itemData == 0 {
						t.Fatal("操作项应使用统一的原生自绘样式")
					}
					if (info.state&3 != 0) != (item.Disabled || item.Header != "") {
						t.Fatal("外观不能改变原来的禁用状态")
					}
					if (info.state&mfChecked != 0) != item.Checked {
						t.Fatal("勾选状态必须保留给系统菜单")
					}
					measure := measureItemStruct{controlType: odtMenu, itemData: info.itemData}
					if !r.measure(&measure) {
						t.Fatal("测量菜单失败")
					}
					width = max(width, int(measure.width))
					height += int(measure.height)
					rows = append(rows, row{info.itemData, measure.height, item.Id, len(item.Children) > 0})
				}
				if height > r.dp(540) {
					t.Fatalf("常用菜单不应过长：%d", height)
				}
				bitmap, pixels, err := createDibSection(width, height)
				if err != nil {
					t.Fatal(err)
				}
				defer procDeleteObject.Call(bitmap)
				dc, _, _ := procCreateCompatibleDC.Call(0)
				defer procDeleteDC.Call(dc)
				previous, _, _ := procSelectObject.Call(dc, bitmap)
				defer procSelectObject.Call(dc, previous)
				palette := headerPalette("brand", "", dark, false, r.systemBackground, r.systemText)
				y := 0
				for _, row := range rows {
					rect := menuRect{top: int32(y), right: int32(width), bottom: int32(y) + int32(row.height)}
					if row.key == 0 {
						fillMenuRect(dc, rect, palette.background)
						line := rect
						line.top += int32(row.height / 2)
						line.bottom = line.top + int32(max(1, r.dp(1)))
						line.left += int32(r.dp(12))
						line.right -= int32(r.dp(12))
						fillMenuRect(dc, line, mixHeaderColor(palette.background, palette.text, .12))
					} else {
						state := uint32(0)
						if row.id == menuSettings {
							state = 1
						}
						if !r.draw(&drawItemStruct{controlType: odtMenu, dc: dc, rect: rect, itemData: row.key, state: state}) {
							t.Fatal("绘制菜单失败")
						}
						// 系统在 WM_DRAWITEM 之后补画子菜单箭头；离屏预览在同一位置表示箭头。
						if row.submenu {
							arrow := menuRect{left: int32(width - r.dp(19)), top: int32(y) + int32(row.height)/2 - int32(r.dp(6)), right: int32(width - r.dp(7))}
							r.glyph(dc, "chevron", arrow, palette.text)
						}
					}
					y += int(row.height)
				}
				gdi32.NewProc("GdiFlush").Call()
				picture := image.NewNRGBA(image.Rect(0, 0, width, height))
				for offset := 0; offset < len(pixels); offset += 4 {
					copy(picture.Pix[offset:offset+4], []byte{pixels[offset+2], pixels[offset+1], pixels[offset], 255})
				}
				if folder := os.Getenv("PROXI_MENU_SNAPSHOTS"); folder != "" {
					if err := os.MkdirAll(folder, 0755); err != nil {
						t.Fatal(err)
					}
					file, err := os.Create(filepath.Join(folder, fmt.Sprintf("tray-menu-dark%v-%d.png", dark, dpi)))
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
