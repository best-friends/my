# SPDX-License-Identifier: AGPL-3.0-only
# Spike #1152: prints the harness results side by side (python3 -I summarize.py results/*.json).
import json
import sys

runs = {}
for path in sys.argv[1:]:
    with open(path) as f:
        d = json.load(f)
    runs[f"{d['engine']}@{d['devicePixelRatio']}"] = d

keys = []
for d in runs.values():
    for k in d:
        if k not in keys and k not in ("engine", "devicePixelRatio", "userAgent"):
            keys.append(k)


def short(v):
    if isinstance(v, dict) and "ratio" in v and "best" in v:
        b = v["best"]
        return f"r={v['ratio']} best(dx={b['dx']},dy={b['dy']},r={b['ratio']})"
    return json.dumps(v, ensure_ascii=False)


for name, d in runs.items():
    print(name, d.get("userAgent"))
for k in keys:
    print(f"== {k}")
    vals = {n: d.get(k) for n, d in runs.items()}
    if all(isinstance(v, dict) for v in vals.values()):
        sub = []
        for v in vals.values():
            for s in v:
                if s not in sub:
                    sub.append(s)
        for s in sub:
            if s in ("axTree", "log"):
                continue
            line = " | ".join(f"{n}: {short(v.get(s))}" for n, v in vals.items())
            print(f"  {s}: {line}")
    else:
        print("  " + " | ".join(f"{n}: {short(v)}" for n, v in vals.items()))
