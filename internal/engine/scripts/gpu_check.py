"""TouchUp's GPU check: what PyTorch sees, and whether it can compute on it.

Prints a short report for the Engines page; always exits 0, so the report
is shown even when the GPU doesn't work.
"""

import platform
import sys

print(f"Python {sys.version.split()[0]} on {platform.system()} {platform.release()} ({platform.machine()})")
try:
    import torch
except Exception as err:  # noqa: BLE001 - the report is the point
    print(f"PyTorch can't be imported: {err}")
    sys.exit(0)

print(f"PyTorch {torch.__version__}, built for CUDA {torch.version.cuda or 'none (CPU-only build)'}")
if not torch.cuda.is_available():
    if torch.version.cuda is None:
        print("Result: this PyTorch is a CPU-only build, so the GPU isn't used. Reinstall the engine.")
    else:
        print("Result: CUDA isn't available: there's no NVIDIA GPU, or its driver is older than "
              f"CUDA {torch.version.cuda} needs. Update the driver, then try again.")
    sys.exit(0)

arches = torch.cuda.get_arch_list()
print(f"Kernels in this build: {' '.join(arches)}")
ok = True
for i in range(torch.cuda.device_count()):
    major, minor = torch.cuda.get_device_capability(i)
    sm = f"sm_{major}{minor}"
    supported = sm in arches or any(a.startswith("compute_") and int(a[8:]) <= major * 10 + minor for a in arches)
    print(f"GPU {i}: {torch.cuda.get_device_name(i)}, compute {major}.{minor} ({sm}), "
          f"{torch.cuda.get_device_properties(i).total_memory / 2**30:.1f} GB"
          + ("" if supported else " - NOT in this build"))
    for dtype in (torch.float32, torch.float16):
        try:
            x = torch.randn(1, 3, 64, 64, device=f"cuda:{i}", dtype=dtype)
            conv = torch.nn.Conv2d(3, 8, 3, padding=1).to(device=f"cuda:{i}", dtype=dtype)
            y = conv(x)
            torch.cuda.synchronize(i)
            finite = bool(torch.isfinite(y).all())
            print(f"  {str(dtype).replace('torch.', '')}: OK" + ("" if finite else " (but produced NaN/Inf)"))
        except Exception as err:  # noqa: BLE001
            ok = False
            print(f"  {str(dtype).replace('torch.', '')}: FAILED - {str(err).strip().splitlines()[0]}")
print("Result: the GPU works with this PyTorch." if ok else "Result: the GPU can't run this PyTorch build.")
