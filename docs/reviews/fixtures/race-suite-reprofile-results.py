#!/usr/bin/env python3
"""Reduce #419 logs: RUN_DIRECTORY BASE_SHA HEAD_SHA HOST_JSON OUTPUT_JSON [PROBES].

Run after race-suite-reprofile.py finishes. Raw profiles/logs stay outside the
repository; keep their hashes, commands, timing tables and case inventories.
"""
from collections import Counter
import hashlib
import json
from pathlib import Path
import re
import statistics
import subprocess
import sys


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def profile_buckets(raw):
    samples, locations_text = raw.split("Locations\n", 1)
    locations = {}
    current = None
    for line in locations_text.split("Mappings", 1)[0].splitlines():
        match = re.match(r"\s*(\d+):", line)
        if match:
            current = int(match[1])
            locations[current] = line
        elif current is not None:
            locations[current] += " " + line
    disjoint, overlapping = Counter(), Counter()
    for line in samples.splitlines():
        match = re.match(r"\s*\d+\s+(\d+):\s*([\d ]+)\s*$", line)
        if not match:
            continue
        weight = int(match[1])
        stack = "\n".join(locations[int(i)] for i in match[2].split())
        preparing = "_Cfunc__sqlite3_prepare_v2_internal " in stack
        bucket = ("connection_setup" if "(*SQLiteDriver).Open " in stack else
                  "statement_prepare" if preparing else
                  "unattributed_external" if "runtime._ExternalCode " in stack else
                  "remaining_test_runtime")
        disjoint[bucket] += weight
        for name, pattern in {
            "goose": "github.com/pressly/goose",
            "self_check": "(*SelfCheckService).RunSelfCheck ",
            "fixture_open": "internal/testdb.Open ",
            "read_only_open": "internal/db.OpenReadOnly ",
            "close": "(*SQLiteConn).Close ",
            "foundation": "(*SelfCheckService).investmentFoundationCheck ",
            "effective_lot_movements": "internal/db.EffectiveLotMovements ",
            "replay_intents": "internal/db.investmentReplayIntentsQuery ",
        }.items():
            if pattern in stack:
                overlapping[name] += weight
                if preparing:
                    overlapping[name + "_prepare"] += weight
        if preparing and any(p in stack for p in (
                "(*SelfCheckService).investmentFoundationCheck ",
                "internal/db.EffectiveLotMovements ", "internal/db.investmentReplayIntentsQuery ")):
            overlapping["target_union_prepare"] += weight
    return dict(total_cpu_s=round(sum(disjoint.values()) / 1e9, 3),
                disjoint_cpu_s={k: round(v / 1e9, 3) for k, v in disjoint.items()},
                overlapping_cpu_s={k: round(v / 1e9, 3) for k, v in overlapping.items()})


def application(path):
    terminal = {}
    package = None
    for line in path.read_text().splitlines():
        event = json.loads(line)
        if event["Action"] not in ("pass", "fail", "skip"):
            continue
        if event.get("Test"):
            terminal[event["Test"]] = event
        else:
            package = event
    assert package and package["Action"] == "pass", path
    assert all(e["Action"] != "fail" for e in terminal.values()), path
    # A t.Run name may itself contain slashes, without emitting an event for
    # every intermediate prefix. Exclude every ancestor, not just the last.
    parents = {"/".join(parts[:index]) for name in terminal
               for parts in [name.split("/")] for index in range(1, len(parts))}
    leaves = {n: e for n, e in terminal.items() if n not in parents}
    names = sorted(leaves)
    return dict(
        package_wall_s=package["Elapsed"], leaves=len(names),
        leaf_work_s=round(sum(e.get("Elapsed", 0) for e in leaves.values()), 3),
        leaf_case_name_sha256=hashlib.sha256(("\n".join(names) + "\n").encode()).hexdigest(),
        outcomes=dict(Counter(e["Action"] for e in leaves.values())),
        skips=sorted(n for n, e in leaves.items() if e["Action"] == "skip"),
        slowest_leaves=[dict(name=n, wall_s=e.get("Elapsed", 0)) for n, e in sorted(
            leaves.items(), key=lambda item: -item[1].get("Elapsed", 0))[:20]],
    ), {n: e.get("Elapsed", 0) for n, e in leaves.items()}


def main():
    directory = Path(sys.argv[1])
    result = dict(issue=419, sources=dict(base=sys.argv[2], head=sys.argv[3]),
                  conditions=json.loads(Path(sys.argv[4]).read_text()),
                  runs=json.loads((directory / "runs.json").read_text()))
    leaf_sets = {}
    leaf_times = {}
    for row in result["runs"]:
        name = row["name"]
        if not name.startswith("full-"):
            assert row["exit"] == 0, row
        log_path = directory / (name + ".log")
        row["log_sha256"] = digest(log_path)
        if name.startswith("app-") or name == "profile-head":
            values, timings = application(log_path)
            names = set(timings)
            row.update(values)
            if name.startswith("app-"):
                label = name.split("-")[1]
                if label in leaf_sets:
                    assert names == leaf_sets[label], name
                leaf_sets[label] = names
                leaf_times.setdefault(label, []).append(timings)
            else:
                assert names == leaf_sets["head"], "Profile must run the complete current suite"
        if name.startswith("full-"):
            log = log_path.read_text()
            assert "(cached)" not in log, name
            row["package_wall_s"] = {
                package: float(elapsed) for _, package, elapsed in re.findall(
                    r"^(ok|FAIL)\s+(rekenraam/\S+)\s+(\d+(?:\.\d+)?)s", log, re.M)}
            row["package_status"] = {
                package: status for status, package, _ in re.findall(
                    r"^(ok|FAIL)\s+(rekenraam/\S+)\s+(\d+(?:\.\d+)?)s", log, re.M)}
            row["timeout"] = "panic: test timed out" in log
            row["phases"] = {
                phase: dict(wall_s=float(wall), user_s=float(user), sys_s=float(system))
                for phase, wall, user, system in re.findall(
                    r"Go phase ([\w-]+): wall=([\d.]+)s user=([\d.]+)s sys=([\d.]+)s", log)}
            assert "race-tests" in row["phases"]
    result["medians_s"] = {
        label: statistics.median(row["package_wall_s"] for row in result["runs"]
                                 if row["name"].startswith("app-" + label))
        for label in ("base", "head")}
    result["leaf_names_added"] = sorted(leaf_sets["head"] - leaf_sets["base"])
    result["leaf_names_removed"] = sorted(leaf_sets["base"] - leaf_sets["head"])
    common = leaf_sets["base"] & leaf_sets["head"]
    result["leaf_work_by_name_set_s"] = {
        label: [dict(common=round(sum(times[n] for n in common), 3),
                     variant_only=round(sum(times[n] for n in times.keys() - common), 3))
                for times in leaf_times[label]]
        for label in ("base", "head")}
    result["leaf_digest_method"] = "sha256 of sorted leaf names joined by newline with trailing newline"
    full = [row for row in result["runs"] if row["name"].startswith("full-")]
    result["adr0016"] = dict(
        investigate=result["medians_s"]["head"] > 210,
        act_packages=sorted(p for p in full[0]["package_wall_s"]
                            if all(row["package_wall_s"].get(p, 0) > 270 for row in full)))
    result["profile"] = dict(sha256=digest(directory / "head.cpu"), reports={})
    for name, options in {
        "cumulative": ["-cum"],
        "flat": [],
        "prepare_callers": ["-cum", "-focus=_Cfunc__sqlite3_prepare_v2_internal",
                            r"-ignore=\(\*SQLiteDriver\).Open"],
        "connection_open": ["-cum", r"-focus=\(\*SQLiteDriver\).Open"],
        "other_native": ["-cum", "-focus=runtime.cgocall",
                         r"-ignore=\(\*SQLiteDriver\).Open|_Cfunc__sqlite3_prepare_v2_internal"],
        "outside_native": ["-cum", "-ignore=runtime.cgocall"],
        "migration": ["-cum", "-focus=pressly/goose"],
        "self_check": ["-cum", "-focus=SelfCheckService.*RunSelfCheck"],
    }.items():
        command = ["go", "tool", "pprof", "-top", "-nodecount=60", *options,
                   str(directory / "head-app.test"), str(directory / "head.cpu")]
        head_tree = next(Path(row["tree"]) for row in result["runs"] if row["name"] == "compile-head")
        report = subprocess.run(command, cwd=head_tree / "backend", check=True, capture_output=True, text=True)
        result["profile"]["reports"][name] = dict(command=command, text=report.stdout)
    for label in ("base", "head"):
        result["sources"][label + "_binary_sha256"] = digest(directory / (label + "-app.test"))
    raw = subprocess.run(["go", "tool", "pprof", "-raw", str(directory / "head-app.test"),
                          str(directory / "head.cpu")], cwd=head_tree / "backend",
                         check=True, capture_output=True, text=True).stdout
    result["profile"]["buckets"] = profile_buckets(raw)
    if len(sys.argv) > 6:
        probes = Path(sys.argv[6])
        result["native_probes"] = dict(runs=json.loads((probes / "probe-runs.json").read_text()))
        for label in ("base", "head"):
            rows = json.loads((probes / ("probe-" + label + ".json")).read_text())
            for row in rows:
                row["open_median_ms"] = statistics.median(row["OpenMS"])
                row["prepare_median_ms"] = statistics.median(row["PrepareMS"]) if row["PrepareMS"] else None
            result["native_probes"][label] = rows
    Path(sys.argv[5]).write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(dict(medians_s=result["medians_s"], adr0016=result["adr0016"]), indent=2))


if __name__ == "__main__":
    main()
