"""TouchUp's runner for PyTorch super-resolution models.

Loads any model spandrel recognizes (ESRGAN, Real-ESRGAN, SwinIR, HAT,
DAT, DRCT, ATD, RealPLKSR, RCAN, ...) and upscales one image, or every
image in a folder, tile by tile so large pictures fit in GPU memory.
Progress is printed as "N%" lines.

    python spandrel_runner.py --model 4x.pth --input in.png --output out.png
    python spandrel_runner.py --model 4x.pth --input frames/ --output upscaled/
"""

import argparse
import math
import os
import sys

import numpy as np
import torch
from PIL import Image

import spandrel

try:  # architectures with non-commercial licenses live in a separate package
    import spandrel_extra_arches

    spandrel_extra_arches.install()
except ImportError:
    pass

IMAGE_EXTS = {".png", ".jpg", ".jpeg", ".webp", ".bmp", ".tif", ".tiff"}


def pick_device(gpu):
    if torch.cuda.is_available():
        return torch.device(f"cuda:{gpu}" if gpu >= 0 else "cuda")
    if getattr(torch.backends, "mps", None) and torch.backends.mps.is_available():
        return torch.device("mps")
    return torch.device("cpu")


def load_model(path, device):
    model = spandrel.ModelLoader(device=device).load_from_file(path)
    if not isinstance(model, spandrel.ImageModelDescriptor):
        sys.exit(f"{os.path.basename(path)} is not an image upscaling model")
    model.eval()
    dtype = torch.float32
    if device.type == "cuda":
        if model.supports_half:
            dtype = torch.float16
        elif model.supports_bfloat16:
            dtype = torch.bfloat16
    model.to(dtype)
    return model, dtype


def pad_to(t, req):
    """Pads a 1xCxHxW tensor to the model's size requirements."""
    _, _, h, w = t.shape
    m = max(1, req.multiple_of)
    ph = max(req.minimum, math.ceil(h / m) * m)
    pw = max(req.minimum, math.ceil(w / m) * m)
    if req.square:
        ph = pw = max(ph, pw)
    if ph == h and pw == w:
        return t
    mode = "reflect" if ph - h < h and pw - w < w else "replicate"
    return torch.nn.functional.pad(t, (0, pw - w, 0, ph - h), mode=mode)


class Progress:
    def __init__(self, total):
        self.total, self.done, self.last = max(1, total), 0, -1

    def step(self):
        self.done += 1
        pct = int(self.done * 100 / self.total)
        if pct != self.last:
            self.last = pct
            print(f"{pct}%", flush=True)


@torch.inference_mode()
def upscale(model, dtype, device, rgb, tile, progress):
    """Upscales an HxWx3 float32 array in [0, 1] by the model's scale."""
    scale = model.scale
    h, w, _ = rgb.shape
    src = torch.from_numpy(np.ascontiguousarray(rgb.transpose(2, 0, 1))).unsqueeze(0)
    out = np.zeros((h * scale, w * scale, 3), dtype=np.float32)
    tile = tile if tile > 0 else max(h, w)
    overlap = 16 if tile < max(h, w) else 0
    for y in range(0, h, tile):
        for x in range(0, w, tile):
            y0, x0 = max(0, y - overlap), max(0, x - overlap)
            y1, x1 = min(h, y + tile + overlap), min(w, x + tile + overlap)
            part = src[:, :, y0:y1, x0:x1].to(device=device, dtype=dtype)
            res = model(pad_to(part, model.size_requirements))
            res = res[:, :, : (y1 - y0) * scale, : (x1 - x0) * scale]
            res = res.float().clamp_(0, 1).squeeze(0).permute(1, 2, 0).cpu().numpy()
            # Keep the tile's own area; the overlap only gave it context.
            ty, tx = (y - y0) * scale, (x - x0) * scale
            th, tw = (min(h, y + tile) - y) * scale, (min(w, x + tile) - x) * scale
            out[y * scale : y * scale + th, x * scale : x * scale + tw] = res[ty : ty + th, tx : tx + tw]
            progress.step()
    return out


def color_fix(out, rgb, scale):
    """Keeps the model's detail but takes color from the original.

    Some models drift on saturated colors. The result's low frequencies
    are swapped for those of the original upscaled smoothly, which leaves
    texture alone and puts every hue back where it was.
    """
    h, w = out.shape[:2]
    k = max(2, scale * 4)
    up = torch.from_numpy(np.ascontiguousarray(out.transpose(2, 0, 1))).unsqueeze(0)
    ref = torch.from_numpy(np.ascontiguousarray(rgb.transpose(2, 0, 1))).unsqueeze(0)
    ref = torch.nn.functional.interpolate(ref, size=(h, w), mode="bicubic", align_corners=False)

    def low(t):
        pooled = torch.nn.functional.avg_pool2d(t, k, stride=k, ceil_mode=True, count_include_pad=False)
        return torch.nn.functional.interpolate(pooled, size=(h, w), mode="bilinear", align_corners=False)

    fixed = (up - low(up) + low(ref)).clamp_(0, 1)
    return fixed.squeeze(0).permute(1, 2, 0).numpy()


def tiles_for(path, tile):
    with Image.open(path) as im:
        w, h = im.size
    if tile <= 0:
        return 1
    return math.ceil(h / tile) * math.ceil(w / tile)


def process(model, dtype, device, src, dst, scale, tile, fix, progress):
    im = Image.open(src)
    alpha = None
    if im.mode in ("RGBA", "LA") or (im.mode == "P" and "transparency" in im.info):
        im = im.convert("RGBA")
        alpha = im.getchannel("A")
    rgb = np.asarray(im.convert("RGB"), dtype=np.float32) / 255.0
    while True:
        try:
            out = upscale(model, dtype, device, rgb, tile, progress)
            break
        except torch.cuda.OutOfMemoryError:
            if tile and tile <= 128:
                raise
            torch.cuda.empty_cache()
            tile = 512 if tile <= 0 else tile // 2
            print(f"Out of GPU memory, retrying with {tile}px tiles", flush=True)
    if fix:
        out = color_fix(out, rgb, model.scale)
    result = Image.fromarray((out * 255.0).round().astype(np.uint8), "RGB")
    target = (rgb.shape[1] * scale, rgb.shape[0] * scale)
    if scale != model.scale:
        result = result.resize(target, Image.LANCZOS)
    if alpha is not None:
        result.putalpha(alpha.resize(result.size, Image.LANCZOS))
    result.save(dst)


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--model", required=True)
    ap.add_argument("--input", required=True, help="an image or a folder of images")
    ap.add_argument("--output", required=True, help="an image or a folder")
    ap.add_argument("--scale", type=int, default=0, help="output scale; defaults to the model's")
    ap.add_argument("--tile", type=int, default=512, help="tile size in pixels; 0 for none")
    ap.add_argument("--gpu", type=int, default=-1)
    ap.add_argument("--no-color-fix", action="store_true", help="keep the model's own colors")
    args = ap.parse_args()

    device = pick_device(args.gpu)
    model, dtype = load_model(args.model, device)
    scale = args.scale or model.scale
    print(f"Loaded {model.architecture.name} {model.scale}x on {device}", flush=True)

    if os.path.isdir(args.input):
        os.makedirs(args.output, exist_ok=True)
        names = sorted(n for n in os.listdir(args.input) if os.path.splitext(n)[1].lower() in IMAGE_EXTS)
        jobs = [(os.path.join(args.input, n), os.path.join(args.output, os.path.splitext(n)[0] + ".png")) for n in names]
    else:
        jobs = [(args.input, args.output)]
    progress = Progress(sum(tiles_for(src, args.tile) for src, _ in jobs))
    for src, dst in jobs:
        process(model, dtype, device, src, dst, scale, args.tile, not args.no_color_fix, progress)


if __name__ == "__main__":
    main()
