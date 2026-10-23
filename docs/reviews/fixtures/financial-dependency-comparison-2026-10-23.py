#!/usr/bin/env python3
"""Reproduce #391's historical comparison without editing the working tree."""

import hashlib
import io
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile


ROOT = Path(__file__).resolve().parents[3]
SOURCE = "b7cd389a"
PROBE = Path(__file__).with_name("financial-dependency-comparison-2026-10-23_test.go.txt")


def remove_fence(path, function):
    source = path.read_text()
    start = source.index("func " + function + "(")
    opening = source.index(" {\n", start) + 2
    end = source.index("\n}\n", opening)
    path.write_text(source[:opening] + "\n\treturn nil" + source[end:])


def run(root, ablated):
    environment = os.environ.copy()
    environment["REKENRAAM_DEPENDENCY_ABLATION"] = "1" if ablated else "0"
    command = [str(root / "scripts/go-backend.sh"), "test", "-race", "-count=1",
               "-timeout=120s", "-v", "-run", "^TestDependencyComparisonAudit", "./internal/app"]
    result = subprocess.run(command, cwd=root / "backend", env=environment,
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    print(result.stdout, end="", flush=True)
    result.check_returncode()


if __name__ == "__main__":
    sha = subprocess.check_output(["git", "rev-parse", SOURCE], cwd=ROOT, text=True).strip()
    archive = subprocess.check_output(["git", "archive", sha, "backend", "scripts", "fixtures"], cwd=ROOT)
    print(f"Source: {sha}", flush=True)
    print(f"Probe SHA-256: {hashlib.sha256(PROBE.read_bytes()).hexdigest()}", flush=True)
    with tempfile.TemporaryDirectory(prefix="rekenraam-391-") as directory:
        snapshot = Path(directory)
        with tarfile.open(fileobj=io.BytesIO(archive)) as bundle:
            bundle.extractall(snapshot, filter="data")
        print(subprocess.check_output(["go", "version"], cwd=snapshot / "backend", text=True).strip(), flush=True)
        (snapshot / "backend/internal/app/dependency_comparison_audit_test.go").write_bytes(PROBE.read_bytes())
        print("BASELINE: both relationship fences intact", flush=True)
        run(snapshot, False)
        remove_fence(snapshot / "backend/internal/db/investment_payment_in_lieu.go",
                     "requirePaymentsInLieuStillEntitledTx")
        remove_fence(snapshot / "backend/internal/db/investment_cash_in_lieu.go",
                     "requireNoCashInLieuSettlesTx")
        print("ABLATION: only the two after-effect/parent fences disabled", flush=True)
        run(snapshot, True)
