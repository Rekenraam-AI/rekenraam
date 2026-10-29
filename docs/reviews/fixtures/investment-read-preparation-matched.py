#!/usr/bin/env python3
"""#445 matched race comparison: BASE_TREE HEAD_TREE OUTPUT.

Run sequentially without competing jobs. Trees must have identical policy and
toolchain; OUTPUT must be fresh. Gate failures are retained and do not prevent
the remaining measurements. Exit nonzero if any measured command fails.
"""
import datetime
import hashlib
import json
import os
from pathlib import Path
import resource
import subprocess
import sys
import time


def main():
    base, head, output = (Path(arg).resolve() for arg in sys.argv[1:])
    output.mkdir(parents=True, exist_ok=True)
    if (output / "runs.json").exists():
        raise SystemExit("Use a fresh output directory")
    if os.environ.get("GOMAXPROCS") or os.environ.get("GOFLAGS"):
        raise SystemExit("Unset GOMAXPROCS and GOFLAGS")
    rows = []
    sources = {}
    for label, tree in (("base", base), ("head", head)):
        paths = sorted(p for p in (tree / "backend").rglob("*")
                       if p.is_file() and p.suffix in (".go", ".sql"))
        sources[label] = {
            str(p.relative_to(tree)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in paths}
    (output / "source-files.json").write_text(json.dumps(sources, indent=2) + "\n")

    def run(name, tree, command, env=None):
        started = datetime.datetime.now(datetime.timezone.utc).isoformat()
        print(started, name, flush=True)
        before = time.monotonic()
        usage_before = resource.getrusage(resource.RUSAGE_CHILDREN)
        with (output / (name + ".log")).open("wb") as log:
            done = subprocess.run(command, cwd=tree, env=dict(os.environ, **(env or {})),
                                  stdout=log, stderr=subprocess.STDOUT)
        usage_after = resource.getrusage(resource.RUSAGE_CHILDREN)
        row = dict(name=name, tree=str(tree), command=command, extra_env=env or {},
                   started_utc=started, wall_s=round(time.monotonic() - before, 3),
                   user_s=round(usage_after.ru_utime - usage_before.ru_utime, 3),
                   sys_s=round(usage_after.ru_stime - usage_before.ru_stime, 3),
                   exit=done.returncode)
        rows.append(row)
        (output / "runs.json").write_text(json.dumps(rows, indent=2) + "\n")
        print(name, "exit", row["exit"], "wall_s", row["wall_s"], flush=True)
        if done.returncode and name.startswith("compile-"):
            raise SystemExit(done.returncode)

    for label, tree in (("base", base), ("head", head)):
        run("compile-" + label, tree,
            ["./scripts/go-backend.sh", "test", "-race", "-c", "-o",
             str(output / (label + "-app.test")), "./internal/app"])
    for number in range(1, 4):
        variants = (("head", head), ("base", base)) if number == 2 else (
            ("base", base), ("head", head))
        for label, tree in variants:
            run(f"app-{label}-{number}", tree / "backend/internal/app",
                ["go", "tool", "test2json", "-t", "-p", "rekenraam/backend/internal/app",
                 str(output / (label + "-app.test")), "-test.v=test2json",
                 "-test.count=1", "-test.timeout=5m"])
    for number in (1, 2):
        variants = (("base", base), ("head", head)) if number == 1 else (
            ("head", head), ("base", base))
        for label, tree in variants:
            run(f"full-{label}-{number}", tree, ["./scripts/test-backend.sh"],
                {"GOFLAGS": "-count=1", "GO_PHASE_TIMINGS": "1"})
    if any(row["exit"] for row in rows):
        raise SystemExit(1)


if __name__ == "__main__":
    main()
