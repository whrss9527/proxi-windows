//go:build windows

package main

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"
)

func TestFloatingHiddenWindowLifecycle(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for count := 0; count < 2; count++ {
		widget, err := newFloatingWindow(nil)
		if err != nil {
			t.Fatal(err)
		}
		window := widget.window
		style, _, _ := user32.NewProc("GetWindowLongPtrW").Call(window, ^uintptr(19))
		if style&0x08000080 != 0x08000080 {
			t.Error("悬浮按钮应为不抢焦点的工具窗口")
		}
		if visible, _, _ := procIsWindowVisible.Call(window); visible != 0 {
			t.Error("测试窗口不应显示到用户桌面")
		}
		widget.close()
		widget.close()
		if floatingWindows[window] != nil {
			t.Error("关闭后不能保留窗口引用")
		}
	}
}

func TestFloatingNativeRendering(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if unsafe.Sizeof(floatingPaintStruct{}) != 72 {
		t.Fatal("Windows 绘制结构的尺寸不正确")
	}
	for _, dpi := range []int{96, 144, 192} {
		for _, dark := range []bool{false, true} {
			r := menuHeaderRendererAtDpi(dpi)
			r.dark, r.highContrast = dark, false
			widget := &floatingWindow{renderer: r, speed: SpeedInfo{Mode: speedSystem, Ready: true, Download: 1258291, Upload: 12800}}
			widget.icon, _ = createIcon(renderTrayIcon(r.dp(28), "brand", iconStateOn, "#16a34a"))
			width, height := r.dp(164), r.dp(44)
			bitmap, pixels, err := createDibSection(width, height)
			if err != nil {
				t.Fatal(err)
			}
			dc, _, _ := procCreateCompatibleDC.Call(0)
			previous, _, _ := procSelectObject.Call(dc, bitmap)
			widget.paint(dc, menuRect{right: int32(width), bottom: int32(height)})
			gdi32.NewProc("GdiFlush").Call()
			picture := image.NewNRGBA(image.Rect(0, 0, width, height))
			colored := 0
			for offset := 0; offset < len(pixels); offset += 4 {
				copy(picture.Pix[offset:offset+4], []byte{pixels[offset+2], pixels[offset+1], pixels[offset], 255})
				if int(pixels[offset]) > int(pixels[offset+2])+20 {
					colored++
				}
				x, y := float64(offset/4%width)+.5, float64(offset/4/width)+.5
				radius := float64(height) / 2
				centerX := math.Max(radius, math.Min(x, float64(width)-radius))
				if math.Hypot(x-centerX, y-radius) > radius {
					picture.Pix[offset+3] = 0
				}
			}
			if colored < r.dp(10) {
				t.Error("左侧应用标识应保持彩色")
			}
			if folder := os.Getenv("PROXI_FLOAT_SNAPSHOTS"); folder != "" {
				_ = os.MkdirAll(folder, 0755)
				file, err := os.Create(filepath.Join(folder, fmt.Sprintf("floating-dark%v-%d.png", dark, dpi)))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(file, picture)
				file.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			procSelectObject.Call(dc, previous)
			procDeleteObject.Call(bitmap)
			procDeleteDC.Call(dc)
			widget.close()
		}
	}
}
