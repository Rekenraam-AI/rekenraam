#!/usr/bin/env python3
"""Characterize #391 candidates in a disposable source snapshot only."""
import hashlib
import io
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[3]
SOURCE = "4ad8373f772eb02b40126f5e9d746f0eb50c9e79"
PROBE = Path(__file__).with_name("financial-dependency-inventory-2026-10-23_test.go.txt")


def replace_once(path, old, new):
    source = path.read_text()
    assert source.count(old) == 1, (path, old)
    path.write_text(source.replace(old, new, 1))


def run(root, ablated):
    environment = os.environ.copy()
    environment["REKENRAAM_INVENTORY_ABLATION"] = "1" if ablated else "0"
    command = [str(root / "scripts/go-backend.sh"), "test", "-race", "-count=1",
               "-timeout=120s", "-v", "-run", "^TestDependencyInventory", "./internal/app"]
    subprocess.run(command, cwd=root / "backend", env=environment, check=True)


if __name__ == "__main__":
    print(f"Source: {SOURCE}", flush=True)
    print(f"Probe SHA-256: {hashlib.sha256(PROBE.read_bytes()).hexdigest()}", flush=True)
    archive = subprocess.check_output(["git", "archive", SOURCE, "backend", "scripts", "fixtures"], cwd=ROOT)
    with tempfile.TemporaryDirectory(prefix="rekenraam-391-inventory-") as directory:
        root = Path(directory)
        with tarfile.open(fileobj=io.BytesIO(archive)) as bundle:
            bundle.extractall(root, filter="data")
        (root / "backend/internal/app/dependency_inventory_test.go").write_bytes(PROBE.read_bytes())
        print(subprocess.check_output(["go", "version"], cwd=root / "backend", text=True).strip(), flush=True)
        print("BASELINE: runtime unchanged", flush=True)
        run(root, False)
        replace_once(root / "backend/internal/app/investment_cash_in_lieu.go",
                     "if settled.AlreadyCorrected && !replacing {", "if false {")
        replace_once(root / "backend/migrations/0024_cash_in_lieu_parent_provenance.sql",
                     "AND (NOT EXISTS (SELECT 1 FROM investment_operations successor\n      WHERE successor.correction_of_operation_id = settled.id)",
                     "AND (1 = 1")
        replace_once(root / "backend/internal/app/investment_reinvestment_correction.go",
                     "if replacement.TransactionDate != operation.EventDate ||\n\t\treplacement.HoldingAccountID != operation.AccountID ||\n\t\treplacement.CommodityID != operation.CommodityID ||\n\t\treplacement.CashCommodityID != operation.CostCommodityID {", "if false {")
        replace_once(root / "backend/internal/app/investment_dividend_correction.go",
                     "if replacement.TransactionDate != operation.EventDate || replacement.CashAccountID != cash.AccountID ||\n\t\treplacement.CashCommodityID != cash.CommodityID {", "if false && cash.AccountID > 0 {")
        replace_once(root / "backend/internal/app/investment_payment_in_lieu.go",
                     "if replacement.TransactionDate != operation.EventDate ||\n\t\treplacement.CashAccountID != cash.AccountID || replacement.CashCommodityID != cash.CommodityID {", "if false && cash.AccountID > 0 {")
        for filename in ["investment_dividend_correction.go", "investment_payment_in_lieu.go"]:
            replace_once(root / "backend/internal/db" / filename,
                         "\n\t\treplacementParams.Spec.TransactionDate != expected.EventDate ||", "")
        print("ABLATION: only CIL entry standing and dividend/payment/reinvestment fixed-field checks removed", flush=True)
        run(root, True)
