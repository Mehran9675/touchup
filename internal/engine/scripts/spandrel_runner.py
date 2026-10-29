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
import traceback

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


def supports(device):
    """Whether this PyTorch build has kernels for a CUDA device: native
    ones for its architecture, or PTX it can compile for it."""
    if device.type != "cuda":
        return True
    i = device.index if device.index is not None else torch.cuda.current_device()
    major, minor = torch.cuda.get_device_capability(i)
    arches = torch.cuda.get_arch_list()
    return f"sm_{major}{minor}" in arches or any(
        a.startswith("compute_") and a[8:].isdigit() and int(a[8:]) <= major * 10 + minor for a in arches)


def describe_device(device):
    if device.type != "cuda":
        return str(device)
    i = device.index if device.index is not None else torch.cuda.current_device()
    major, minor = torch.cuda.get_device_capability(i)
    return f"{torch.cuda.get_device_name(i)} (CUDA {torch.version.cuda}, compute {major}.{minor})"


# What a GPU raises when it can't run a model: CUDA errors are
# RuntimeErrors, and newer PyTorch has its own AcceleratorError.
# FloatingPointError is a model overflowing in reduced precision.
GPU_ERRORS = (RuntimeError, FloatingPointError) + ((torch.AcceleratorError,) if hasattr(torch, "AcceleratorError") else ())


class Upscaler:
    """A loaded model that steps down when the GPU can't run it: from
    reduced to full precision, then from the GPU to the CPU.

    The weights stay in float32 and reduced precision comes from autocast,
    which casts each operation's inputs. Casting the weights instead breaks
    models that build tensors as they run, such as the attention masks of
    DRCT, ATD, HAT and SwinIR, which are float32 whatever the weights are."""

    def __init__(self, path, device):
        self.model = spandrel.ModelLoader(device=torch.device("cpu")).load_from_file(path)
        if not isinstance(self.model, spandrel.ImageModelDescriptor):
            raise ValueError(f"{os.path.basename(path)} is not an image upscaling model")
        self.model.eval()
        self.place(device, self.best_dtype(device))

    def best_dtype(self, device):
        if device.type == "cuda":
            if self.model.supports_half:
                return torch.float16
            if self.model.supports_bfloat16:
                return torch.bfloat16
        return torch.float32

    def place(self, device, dtype):
        self.device, self.dtype = device, dtype
        self.model.to(device)

    def precision(self):
        return {torch.float16: "half precision", torch.bfloat16: "bfloat16"}.get(self.dtype, "full precision")

    def step_down(self, err):
        """Moves to a safer setting after err, or returns False when there
        is none left."""
        reason = str(err).strip().splitlines()[0] if str(err).strip() else type(err).__name__
        if self.device.type == "cpu":
            return False
        if self.dtype != torch.float32:
            print(f"The GPU failed in {self.precision()} ({reason}); retrying in full precision", flush=True)
            self.place(self.device, torch.float32)
        else:
            print(f"The GPU failed ({reason}); continuing on the CPU, which is much slower", flush=True)
            if self.device.type == "cuda":
                torch.cuda.empty_cache()
            self.place(torch.device("cpu"), torch.float32)
        return True


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
def upscale(up, rgb, tile, progress):
    """Upscales an HxWx3 float32 array in [0, 1] by the model's scale."""
    model, device = up.model, up.device
    autocast = torch.autocast(device.type, dtype=up.dtype, enabled=up.dtype != torch.float32)
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
            part = src[:, :, y0:y1, x0:x1].to(device=device)
            with autocast:
                res = model(pad_to(part, model.size_requirements))
            res = res[:, :, : (y1 - y0) * scale, : (x1 - x0) * scale]
            if not torch.isfinite(res).all():
                raise FloatingPointError(f"the model's output overflowed in {up.precision()}")
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


def process(up, src, dst, scale, tile, fix, progress):
    im = Image.open(src)
    alpha = None
    if im.mode in ("RGBA", "LA") or (im.mode == "P" and "transparency" in im.info):
        im = im.convert("RGBA")
        alpha = im.getchannel("A")
    rgb = np.asarray(im.convert("RGB"), dtype=np.float32) / 255.0
    while True:
        done = progress.done
        try:
            out = upscale(up, rgb, tile, progress)
            break
        except torch.cuda.OutOfMemoryError:
            progress.done = done
            torch.cuda.empty_cache()
            if tile and tile <= 128:
                raise
            tile = 512 if tile <= 0 else tile // 2
            print(f"Out of GPU memory, retrying with {tile}px tiles", flush=True)
        except GPU_ERRORS as err:
            progress.done = done
            if not up.step_down(err):
                raise
    if fix:
        out = color_fix(out, rgb, up.model.scale)
    result = Image.fromarray((out * 255.0).round().astype(np.uint8), "RGB")
    target = (rgb.shape[1] * scale, rgb.shape[0] * scale)
    if scale != up.model.scale:
        result = result.resize(target, Image.LANCZOS)
    if alpha is not None:
        result.putalpha(alpha.resize(result.size, Image.LANCZOS))
    result.save(dst)


def explain(err):
    """A one-line reason for a failure, with what to do where it's known."""
    msg = str(err).strip().splitlines()[0] if str(err).strip() else type(err).__name__
    low = msg.lower()
    if "no kernel image" in low or "not compatible with the current pytorch" in low:
        return msg + ". This GPU isn't supported by the installed PyTorch build."
    if isinstance(err, torch.cuda.OutOfMemoryError) or "out of memory" in low:
        return "The GPU ran out of memory even with small tiles. Try a smaller model or close other GPU apps."
    if isinstance(err, FileNotFoundError):
        return f"File not found: {err.filename}"
    return f"{type(err).__name__}: {msg}"


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
    if not supports(device):
        print(f"This PyTorch (CUDA {torch.version.cuda}) has no kernels for {describe_device(device)}; "
              "running on the CPU. Reinstall the engine to get a PyTorch for this GPU.", flush=True)
        device = torch.device("cpu")
    up = Upscaler(args.model, device)
    scale = args.scale or up.model.scale
    print(f"Loaded {up.model.architecture.name} {up.model.scale}x on {describe_device(up.device)} "
          f"in {up.precision()}, PyTorch {torch.__version__}", flush=True)

    if os.path.isdir(args.input):
        os.makedirs(args.output, exist_ok=True)
        names = sorted(n for n in os.listdir(args.input) if os.path.splitext(n)[1].lower() in IMAGE_EXTS)
        jobs = [(os.path.join(args.input, n), os.path.join(args.output, os.path.splitext(n)[0] + ".png")) for n in names]
    else:
        jobs = [(args.input, args.output)]
    progress = Progress(sum(tiles_for(src, args.tile) for src, _ in jobs))
    for src, dst in jobs:
        process(up, src, dst, scale, args.tile, not args.no_color_fix, progress)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)
    except Exception as err:  # the last line is what TouchUp shows
        traceback.print_exc()
        print(f"Error: {explain(err)}", flush=True)
        sys.exit(1)
