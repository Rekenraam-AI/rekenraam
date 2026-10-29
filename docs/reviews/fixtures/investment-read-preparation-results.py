#!/usr/bin/env python3
"""Reduce #445 evidence: RAW_DIRECTORY OUTPUT_JSON.

Use after investment-read-preparation-matched.py completes. SQL/fixture probe
inputs live under RAW_DIRECTORY as described by the review's reproduction steps.
No database work or timing command runs in this reducer.
"""
from collections import Counter
import hashlib
import json
from pathlib import Path
import re
import runpy
import statistics
import sys


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    root, output = map(Path, sys.argv[1:])
    matched = root / "matched"
    application = runpy.run_path(str(Path(__file__).with_name(
        "race-suite-reprofile-results.py")))["application"]
    result = dict(issue=445, conditions=json.loads((root / "host.json").read_text()))
    if (root / "host-spot-check.json").exists():
        result["host_process_spot_check"] = json.loads((root / "host-spot-check.json").read_text())
    inventory = {}
    for path in sorted((root / "final-evidence").glob("*-queries.json")):
        for row in json.loads(path.read_text()):
            key = hashlib.sha256(row["sql"].encode()).hexdigest()
            item = inventory.setdefault(key, dict(sql=row["sql"], phases=set(), callers=set()))
            item["phases"].add(row["phase"])
            item["callers"].add(row["caller"])
    for key, row in inventory.items():
        row["phases"], row["callers"] = sorted(row["phases"]), sorted(row["callers"])
    result["original_sql_counts"] = dict(distinct=len(inventory), memberships=dict(Counter(
        phase for row in inventory.values() for phase in row["phases"])))
    result["sql_inventory"] = inventory

    def fixtures(directory):
        return {path.stem.removesuffix("-info"): dict(json.loads(path.read_text()),
                    sqlite_sha256=digest(path.with_name(path.name.replace("-info.json", ".sqlite"))),
                    captured_queries_sha256=digest(path.with_name(path.name.replace("-info", "-queries"))))
                for path in sorted(directory.glob("*-info.json"))}

    result["fixtures"] = fixtures(root / "final-evidence")
    result["initial_fixtures"] = fixtures(root / "evidence")
    for number in range(1, 4):
        for fixture, values in result["fixtures"].items():
            assert digest(root / f"accepted-query-{number}" / (fixture + ".sqlite")) == values["sqlite_sha256"]
    result["retained_tool_source_sha256"] = {path.name: digest(path) for path in sorted(
        Path(__file__).parent.glob("investment-read-preparation-*"))
        if path.suffix in (".py", ".patch")}

    def compact(row):
        row = dict(row)
        if "sql" in row:
            sql = row.pop("sql")
            key = hashlib.sha256(sql.encode()).hexdigest()
            inventory.setdefault(key, dict(sql=sql, phases=[row["phase"]], callers=[row["caller"]]))
            row["sql_sha256"] = key
        row.pop("location", None)  # Capture archive path is not source identity.
        return row

    result["initial_inventory_measurements"] = [compact(row) for row in json.loads(
        (root / "baseline-sql.json").read_text())]
    result["query_rotations"] = []
    sql_hashes = {}
    for label in ("base", "head"):
        text = (root / label / "backend/internal/db/investment_replay_intents.go").read_text()
        sql = text.split("transfers, err := reader.QueryContext(ctx, `", 1)[1].split("`", 1)[0]
        sql_hashes[label] = hashlib.sha256(sql.encode()).hexdigest()
    result["changed_query_sql_sha256"] = sql_hashes
    for number in range(1, 4):
        rows = [compact(row) for row in json.loads(
            (root / f"accepted-query-{number}.json").read_text())]
        medians = {label: statistics.median(row["prepare_median_us"] for row in rows
                   if row["caller"].endswith(":" + label)) for label in ("base", "head")}
        pairs = {}
        for row in rows:
            label = row["caller"].rsplit(":", 1)[1]
            assert row["sql_sha256"] == sql_hashes[label]
            pairs.setdefault((row["fixture"], tuple(row["args"])), {})[
                label] = row
        for pair in pairs.values():
            for field in ("rows", "rows_sha256"):
                assert pair["base"][field] == pair["head"][field], (number, pair)
        result["query_rotations"].append(dict(number=number, rows=rows,
            preparation_median_us=medians,
            reduction_percent=100 * (1 - medians["head"] / medians["base"])))
    plan_rows = json.loads((root / "final-plans.json").read_text())
    proposed_suffix = ":two" if any(r["caller"].endswith(":two") for r in plan_rows) else ":head"
    result["query_plans"] = [row for row in plan_rows
                             if row["caller"].endswith((":base", proposed_suffix))]
    for row in result["query_plans"]:
        row["caller"] = row["caller"].replace(":two", ":head")
    plans = {}
    for row in result["query_plans"]:
        plans.setdefault((row["fixture"], tuple(row["args"])), {})[
            row["caller"].rsplit(":", 1)[1]] = row["plan"]
    assert plans.keys() == pairs.keys()
    assert all(p["base"] == p["head"] for p in plans.values())
    sources = json.loads((matched / "source-files.json").read_text())
    result["source_files"] = sources
    result["source_changes"] = sorted(path for path in sources["base"] | sources["head"]
        if sources["base"].get(path) != sources["head"].get(path))
    assert result["source_changes"] == ["backend/internal/db/investment_replay_intents.go"]
    runs = json.loads((matched / "runs.json").read_text())
    leaf_names = None
    for row in runs:
        path = matched / (row["name"] + ".log")
        row["log_sha256"] = digest(path)
        if row["name"].startswith("app-"):
            values, timings = application(path)
            row.update(values)
            if leaf_names is None:
                leaf_names = set(timings)
            assert leaf_names == set(timings), row["name"]
        if row["name"].startswith("full-"):
            text = path.read_text()
            assert "(cached)" not in text
            row["packages"] = {package: dict(status=status, wall_s=float(seconds))
                for status, package, seconds in re.findall(
                    r"^(ok|FAIL)\s+(rekenraam/\S+)\s+(\d+(?:\.\d+)?)s", text, re.M)}
            row["timeout"] = "panic: test timed out" in text
            row["named_test_failures"] = re.findall(r"^--- FAIL: (.+)$", text, re.M)
            row["race_detector_warning"] = "WARNING: DATA RACE" in text
            if row["exit"]:
                lines = text.splitlines()
                start = next((i for i, line in enumerate(lines) if line.startswith(
                    ("panic:", "--- FAIL:", "WARNING: DATA RACE"))), max(0, len(lines) - 60))
                row["failure_excerpt"] = lines[start:start + 60]
            row["phases"] = {phase: dict(wall_s=float(wall), user_s=float(user), sys_s=float(system))
                for phase, wall, user, system in re.findall(
                    r"Go phase ([\w-]+): wall=([\d.]+)s user=([\d.]+)s sys=([\d.]+)s", text)}
    result["runs"] = runs
    result["app_median_s"] = {label: statistics.median(row["package_wall_s"] for row in runs
        if row["name"].startswith("app-" + label)) for label in ("base", "head")}
    gates = [row for row in runs if row["name"].startswith("full-head")]
    assert len(gates) == 2
    app = "rekenraam/backend/internal/app"
    result["adr0016"] = dict(investigate=result["app_median_s"]["head"] > 210,
        current_pair_act_packages=sorted(package for package in gates[0]["packages"]
            if all(row["packages"].get(package, {}).get("wall_s", 0) > 270 for row in gates)),
        act_cleared=all(not row["timeout"] and row["packages"][app]["wall_s"] < 270 for row in gates),
        thirty_percent_headroom=all(not row["timeout"] and row["packages"][app]["wall_s"] <= 210 for row in gates))
    result["binary_sha256"] = {label: digest(matched / (label + "-app.test"))
                              for label in ("base", "head")}
    output.write_text(json.dumps(result, indent=2) + "\n")


if __name__ == "__main__":
    main()
