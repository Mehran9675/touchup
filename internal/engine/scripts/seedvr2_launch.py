"""TouchUp's launcher for SeedVR2's command line.

Runs src/inference_cli.py with the arguments it's given, after taking
out its own, which SeedVR2 has no flags for:

    --touchup-chunk-frames N  frames to hold in memory at once, rounded to
                              a whole number of batches (--chunk_size)
    --touchup-vae-memory GB   memory each VAE convolution may use before
                              it's split into pieces (SeedVR2's is 0.5)
    --touchup-cudnn-tune      let cuDNN time its kernels and keep the
                              fastest for these sizes

    python seedvr2_launch.py src/inference_cli.py video.mp4 --batch_size 21 ...
"""

import os
import runpy
import sys


def take(argv, flag, has_value=True):
    """Removes flag (and its value) from argv, returning the value, True
    for a present switch, or None."""
    if flag not in argv:
        return None
    i = argv.index(flag)
    if not has_value:
        del argv[i]
        return True
    value = argv[i + 1] if i + 1 < len(argv) else None
    del argv[i : i + 2]
    return value


def arg(argv, flag, default):
    if flag in argv:
        i = argv.index(flag)
        if i + 1 < len(argv):
            return argv[i + 1]
    return default


def aligned_chunk(frames, batch, overlap):
    """A chunk size that splits into whole batches. Each chunk after the
    first starts with `overlap` frames of the last one, and batches of
    `batch` frames overlap by as many, so every batch adds batch - overlap
    new frames: chunks that are a multiple of that waste nothing."""
    step = batch - overlap if batch > overlap else batch
    return max(step, round(frames / step) * step)


def patch_vae_memory(gigabytes):
    from src.models.video_vae_v3.modules import attn_video_vae

    patched = 0
    for cls in vars(attn_video_vae).values():
        if isinstance(cls, type) and "set_memory_limit" in vars(cls):
            original = cls.set_memory_limit

            def set_memory_limit(self, conv_max_mem=None, norm_max_mem=None, _original=original):
                _original(self, conv_max_mem=gigabytes, norm_max_mem=gigabytes)

            cls.set_memory_limit = set_memory_limit
            patched += 1
    if patched:
        print(f"TouchUp: VAE steps may use {gigabytes:g} GB before splitting", flush=True)


def tune_cudnn():
    import torch

    torch.backends.cudnn.benchmark = True
    # SeedVR2 calls cuDNN directly for 3D convolutions on recent PyTorch,
    # with tuning off; turn it on there too.
    original = torch.cudnn_convolution

    def cudnn_convolution(*args, benchmark=False, **kwargs):
        return original(*args, benchmark=True, **kwargs)

    torch.cudnn_convolution = cudnn_convolution
    print("TouchUp: cuDNN tuning on", flush=True)


def main():
    argv = sys.argv[1:]
    if not argv:
        sys.exit(__doc__)
    cli = os.path.abspath(argv.pop(0))

    chunk = take(argv, "--touchup-chunk-frames")
    vae_memory = take(argv, "--touchup-vae-memory")
    tune = take(argv, "--touchup-cudnn-tune", has_value=False)

    if chunk is not None:
        batch = int(arg(argv, "--batch_size", "5"))
        overlap = int(arg(argv, "--temporal_overlap", "0"))
        size = aligned_chunk(int(chunk), batch, overlap)
        argv += ["--chunk_size", str(size)]
        print(f"TouchUp: {size} frames per chunk, in batches of {batch}", flush=True)

    # SeedVR2 picks the GPU by setting CUDA_VISIBLE_DEVICES before torch
    # starts CUDA, and the patches below import modules that start it, so
    # it's set here first. SeedVR2 leaves a set one alone.
    device = arg(argv, "--cuda_device", None)
    if device is not None and "," not in device and "CUDA_VISIBLE_DEVICES" not in os.environ:
        os.environ["CUDA_VISIBLE_DEVICES"] = device.strip()

    # Imports below need SeedVR2's folder first on the path, as running
    # its script directly would give.
    sys.path.insert(0, os.path.dirname(cli))
    if tune:
        tune_cudnn()
    if vae_memory is not None:
        patch_vae_memory(float(vae_memory))

    sys.argv = [cli] + argv
    runpy.run_path(cli, run_name="__main__")


if __name__ == "__main__":
    main()
