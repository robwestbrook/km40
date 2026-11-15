"""Regenerate the home-screen icons in web/static (needs Pillow).

    python3 tools/make-icons.py

A sun behind a cloud on the dashboard's dark blue. The artwork is drawn once at
1024 px, kept inside the central 60% so Android may crop it to any mask shape,
and downscaled. Icons are full-bleed squares: iOS rounds the corners itself and
paints transparent areas black.
"""
from pathlib import Path
from PIL import Image, ImageDraw

OUT = Path(__file__).resolve().parent.parent / "web" / "static"
S = 1024


def draw() -> Image.Image:
    img = Image.new("RGB", (S, S))
    px = img.load()
    for y in range(S):  # vertical gradient, #10203a at the top to #070b14
        t = y / (S - 1)
        c = tuple(round(a + (b - a) * t) for a, b in ((0x10, 0x07), (0x20, 0x0B), (0x3A, 0x14)))
        for x in range(S):
            px[x, y] = c
    d = ImageDraw.Draw(img)
    d.ellipse((470, 250, 770, 550), fill="#fbbf24")  # sun
    cloud = "#e2e8f0"
    d.rounded_rectangle((250, 560, 760, 720), radius=80, fill=cloud)
    d.ellipse((290, 450, 510, 670), fill=cloud)
    d.ellipse((440, 400, 700, 660), fill=cloud)
    return img


if __name__ == "__main__":
    OUT.mkdir(parents=True, exist_ok=True)
    art = draw()
    for name, size in (("apple-touch-icon.png", 180), ("icon-192.png", 192), ("icon-512.png", 512)):
        art.resize((size, size), Image.LANCZOS).save(OUT / name, optimize=True)
        print("wrote", OUT / name)
