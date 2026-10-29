"""TouchUp's start-up hook for Python engines.

Python runs this at start-up because TouchUp puts its folder on
PYTHONPATH. With TOUCHUP_STRICT_GPU_MEMORY=1 it keeps PyTorch within the
GPU memory that's free when CUDA starts. Without it, Windows drivers let
a GPU that runs out borrow system memory, which is many times slower: a
step that takes seconds can take many minutes. With the cap, the engine
fails at once with "out of memory" and says so.

It changes nothing until the engine imports torch, and then only waits
for CUDA to start, so engines choose their devices as they normally do.
"""

import os
import sys

_MARGIN = 256 * 1024 * 1024  # left for the CUDA context and other apps


def _cap(torch):
    def apply():
        try:
            for i in range(torch.cuda.device_count()):
                free, total = torch.cuda.mem_get_info(i)
                fraction = max(0.05, min(1.0, (free - _MARGIN) / total))
                torch.cuda.set_per_process_memory_fraction(fraction, i)
                print(f"TouchUp: GPU {i} limited to the {fraction * total / 2**30:.2f} GB free; "
                      "it fails instead of spilling into system memory", flush=True)
        except Exception as err:  # never stop an engine over this
            print(f"TouchUp: couldn't limit GPU memory: {err}", flush=True)

    try:
        torch.cuda._lazy_call(apply)
    except Exception as err:
        print(f"TouchUp: couldn't limit GPU memory: {err}", flush=True)


class _AfterTorch:
    """Finds nothing itself; it wraps torch's loader so _cap runs once
    torch has been imported."""

    def find_spec(self, name, path=None, target=None):
        if name != "torch":
            return None
        sys.meta_path.remove(self)
        import importlib.util

        spec = importlib.util.find_spec(name)
        if spec is None or spec.loader is None:
            return spec
        exec_module = spec.loader.exec_module

        def run(module):
            exec_module(module)
            _cap(module)

        spec.loader.exec_module = run
        return spec


if os.environ.get("TOUCHUP_STRICT_GPU_MEMORY") == "1":
    if "torch" in sys.modules:
        _cap(sys.modules["torch"])
    else:
        sys.meta_path.insert(0, _AfterTorch())
