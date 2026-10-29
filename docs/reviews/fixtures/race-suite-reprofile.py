#!/usr/bin/env python3
"""#419 sequential measurement: BASE_TREE HEAD_TREE OUTPUT.

Use clean source trees, with no competing test/build jobs. The base may be a
git archive. Both trees must use the same toolchain. Raw logs and the CPU
profile remain in OUTPUT; publish the reduced evidence alongside the review.
No result cache, CPU cap, database setting or shared-gate limit is changed.
"""
import datetime
import json
import os
from pathlib import Path
import subprocess
import sys
import time


def main():
    base, head, output = (Path(arg).resolve() for arg in sys.argv[1:])
    output.mkdir(parents=True, exist_ok=True)
    manifest = output / "runs.json"
    if manifest.exists():
        raise SystemExit("Use a fresh output directory; never mix measurement sources")
    if os.environ.get("GOMAXPROCS") or os.environ.get("GOFLAGS"):
        raise SystemExit("Unset GOMAXPROCS and GOFLAGS for the matched measurement")
    runs = []

    def run(name, tree, args, extra_env=None, allow_failure=False):
        started = datetime.datetime.now(datetime.timezone.utc).isoformat()
        print(started, name, flush=True)
        with (output / (name + ".log")).open("wb") as log:
            before = time.monotonic()
            result = subprocess.run(
                args, cwd=tree, env=dict(os.environ, **(extra_env or {})),
                stdout=log, stderr=subprocess.STDOUT,
            )
        row = dict(name=name, tree=str(tree), command=args, started_utc=started,
                   wall_s=round(time.monotonic() - before, 3), exit=result.returncode)
        runs.append(row)
        manifest.write_text(json.dumps(runs, indent=2) + "\n")
        print(name, "exit", row["exit"], "wall_s", row["wall_s"], flush=True)
        if result.returncode and not allow_failure:
            raise SystemExit(result.returncode)

    for label, tree in (("base", base), ("head", head)):
        run("compile-" + label, tree,
            ["./scripts/go-backend.sh", "test", "-race", "-c", "-o",
             str(output / (label + "-app.test")), "./internal/app"])
    for number in range(1, 4):
        # Interleave sources, reversing the middle pair to limit order bias.
        variants = (("head", head), ("base", base)) if number == 2 else (
            ("base", base), ("head", head))
        for label, tree in variants:
            run(f"app-{label}-{number}", tree / "backend/internal/app",
                ["go", "tool", "test2json", "-t", "-p", "rekenraam/backend/internal/app",
                 str(output / (label + "-app.test")), "-test.v=test2json",
                 "-test.count=1", "-test.timeout=5m"])
    for number in (1, 2):
        run(f"full-head-{number}", head, ["./scripts/test-backend.sh"],
            {"GOFLAGS": "-count=1", "GO_PHASE_TIMINGS": "1"}, allow_failure=True)
    run("profile-head", head / "backend/internal/app",
        ["go", "tool", "test2json", "-t", "-p", "rekenraam/backend/internal/app",
         str(output / "head-app.test"), "-test.v=test2json", "-test.count=1",
         "-test.timeout=5m", "-test.cpuprofile=" + str(output / "head.cpu")])
    if any(row["exit"] for row in runs):
        raise SystemExit(1)


if __name__ == "__main__":
    main()
