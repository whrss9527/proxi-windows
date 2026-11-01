#!/usr/bin/env python3
"""生成程序图标 assets/app.ico、文档用的 docs/logo.png 和安装程序欢迎页左侧的 assets/installer-side.bmp。

图标是蓝紫色圆角方块上的连接与分流图形，与 web/brand.svg 及 brand.go 保持一致。
应用标识不表示开关状态；托盘在其上叠加状态角标，或按用户选择显示开关。

用法：python3 tools/make_icons.py（需要 Pillow）
"""
import os

from PIL import Image, ImageDraw, ImageFilter

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SIZES = [16, 20, 24, 32, 40, 48, 64, 128, 256]
SUPERSAMPLING = 8


def vertical_gradient(size, top, bottom):
    gradient = Image.new("RGBA", (1, size))
    for y in range(size):
        ratio = y / max(1, size - 1)
        gradient.putpixel((0, y), tuple(round(top[i] + (bottom[i] - top[i]) * ratio) for i in range(3)) + (255,))
    return gradient.resize((size, size))


def render(size):
    canvas_size = size * SUPERSAMPLING
    canvas = Image.new("RGBA", (canvas_size, canvas_size), (0, 0, 0, 0))

    unit = canvas_size / 64
    margin = 4 * unit
    radius = 15 * unit
    mask = Image.new("L", (canvas_size, canvas_size), 0)
    ImageDraw.Draw(mask).rounded_rectangle([margin, margin, canvas_size - margin, canvas_size - margin], radius=radius, fill=255)
    tile = vertical_gradient(canvas_size, (68, 137, 255), (91, 77, 232))
    canvas.paste(tile, (0, 0), mask)

    # 一路连接分成两条平滑的路径；使用圆点和曲线表达连接，不使用字母。
    draw = ImageDraw.Draw(canvas)
    for direction in (-1, 1):
        points = [(18, 32), (24, 32)]
        for step in range(1, 33):
            t = step / 32
            u = 1 - t
            points.append((24*u**3 + 34*3*u*u*t + 32*3*u*t*t + 42*t**3, 32 + direction*12*(3*t*t-2*t**3)))
        points.append((46, 32 + direction*12))
        draw.line([(x*unit, y*unit) for x,y in points], fill="white", width=round(5*unit), joint="curve")
    for x,y,radius,color in [(18,32,6,"white"),(46,20,5,"white"),(46,44,6,(128,241,216,255))]:
        draw.ellipse([(x-radius)*unit,(y-radius)*unit,(x+radius)*unit,(y+radius)*unit],fill=color)

    return canvas.resize((size, size), Image.LANCZOS)


def render_installer_side():
    """安装程序欢迎页和完成页左侧的图（164×314，NSIS 要求 24 位 BMP）：浅色渐变底，上部是程序图标和一圈柔光。"""
    width, height = 164, 314
    scale = 4
    canvas = Image.new("RGB", (width * scale, height * scale))
    top, bottom = (232, 240, 255), (243, 236, 255)
    draw = ImageDraw.Draw(canvas)
    for y in range(height * scale):
        ratio = y / (height * scale - 1)
        draw.line([(0, y), (width * scale, y)], fill=tuple(round(top[i] + (bottom[i] - top[i]) * ratio) for i in range(3)))
    # 图标后面的柔光
    glow = Image.new("L", canvas.size, 0)
    center = (width * scale // 2, round(height * scale * 0.36))
    radius = 70 * scale
    ImageDraw.Draw(glow).ellipse([center[0] - radius, center[1] - radius, center[0] + radius, center[1] + radius], fill=110)
    glow = glow.filter(ImageFilter.GaussianBlur(28 * scale))
    canvas.paste(Image.new("RGB", canvas.size, (255, 255, 255)), (0, 0), glow)
    icon = render(88 * scale // 2).resize((88 * scale, 88 * scale), Image.LANCZOS)
    canvas.paste(icon, (center[0] - icon.width // 2, center[1] - icon.height // 2), icon)
    return canvas.resize((width, height), Image.LANCZOS)


def main():
    render_installer_side().save(os.path.join(ROOT, "assets", "installer-side.bmp"))
    frames = [render(size) for size in SIZES]
    largest = frames[-1]
    largest.save(os.path.join(ROOT, "assets", "app.ico"), sizes=[(size, size) for size in SIZES], append_images=frames[:-1])
    os.makedirs(os.path.join(ROOT, "docs"), exist_ok=True)
    render(128).save(os.path.join(ROOT, "docs", "logo.png"))
    print("已生成 assets/app.ico、docs/logo.png 和 assets/installer-side.bmp")


if __name__ == "__main__":
    main()
