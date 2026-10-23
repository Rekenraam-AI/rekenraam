#!/usr/bin/env python3
"""Pin ci.yml's event policy: one full run per validation context (#383).

Reads the workflow as text (no YAML dependency on the runner) and evaluates its
triggers and concurrency group for focused positive and negative event cases.
"""
import fnmatch
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parent.parent
WORKFLOW = ROOT / ".github/workflows/ci.yml"
# Required status checks on main's branch protection (strict), as configured on
# 2026-10-23. Renaming one of these jobs leaves merges blocked on a check that
# never reports.
REQUIRED_CHECKS = {"Backend Tests", "Frontend Check", "Integrated Build"}


def blocks(lines, indent):
    """Map each key at exactly `indent` spaces to the lines nested under it."""
    result, key = {}, None
    for line in lines:
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        depth = len(line) - len(line.lstrip(" "))
        if depth < indent:
            key = None
        elif depth == indent:
            key, _, value = line.strip().partition(":")
            result[key] = [value.strip()] if value.strip() else []
        elif key is not None:
            result[key].append(line)
    return result


def scalar(lines, name, indent):
    for line in lines:
        match = re.fullmatch(" " * indent + name + r":\s*(.*)", line)
        if match:
            return match.group(1).strip().strip('"')
    return None


TOP = blocks(WORKFLOW.read_text().splitlines(), 0)
EVENTS = blocks(TOP["on"], 2)
JOBS = blocks(TOP["jobs"], 2)


def branch_filter(event):
    value = scalar(EVENTS[event], "branches", 4)
    if value is None:
        return None
    return [item.strip().strip('"') for item in value.strip("[]").split(",")]


def triggers(event, ref):
    if event not in EVENTS:
        return False
    if event != "push":
        return True
    if not ref.startswith("refs/heads/"):
        return False
    branches = branch_filter("push")
    return branches is None or any(fnmatch.fnmatchcase(ref[len("refs/heads/"):], b) for b in branches)


def group(event, ref):
    template = scalar(TOP["concurrency"], "group", 2)
    values = {"github.workflow": "CI", "github.event_name": event, "github.ref": ref}
    return re.sub(r"\$\{\{\s*([\w.]+)\s*\}\}", lambda m: values[m.group(1)], template)


class EventPolicy(unittest.TestCase):
    def test_each_context_triggers_once(self):
        self.assertTrue(triggers("push", "refs/heads/main"))
        self.assertTrue(triggers("pull_request", "refs/pull/353/merge"))
        self.assertTrue(triggers("workflow_dispatch", "refs/heads/feature/x"))

    def test_branch_push_without_pull_request_does_not_run(self):
        # The duplicate pair from #383: a dependabot branch push beside its PR.
        self.assertFalse(triggers("push", "refs/heads/dependabot/npm_and_yarn/svelte-check-4.7.7"))
        self.assertFalse(triggers("push", "refs/heads/fix/262-mfa-settings-reauth-budget"))
        self.assertFalse(triggers("push", "refs/tags/v0.1.0"))

    def test_newer_run_cancels_only_its_own_context(self):
        self.assertEqual(group("pull_request", "refs/pull/7/merge"), group("pull_request", "refs/pull/7/merge"))
        self.assertEqual(group("push", "refs/heads/main"), group("push", "refs/heads/main"))
        contexts = {
            group("push", "refs/heads/main"),
            group("workflow_dispatch", "refs/heads/main"),
            group("pull_request", "refs/pull/7/merge"),
            group("pull_request", "refs/pull/8/merge"),
        }
        self.assertEqual(len(contexts), 4)
        self.assertEqual(scalar(TOP["concurrency"], "cancel-in-progress", 2), "true")

    def test_required_checks_always_report(self):
        names = {scalar(lines, "name", 4) for lines in JOBS.values()}
        self.assertLessEqual(REQUIRED_CHECKS, names)
        for job, lines in JOBS.items():
            self.assertIsNone(scalar(lines, "if", 4), f"job {job} can skip a required check")
        for event in ("push", "pull_request"):
            for key in ("paths", "paths-ignore", "branches-ignore"):
                self.assertIsNone(scalar(EVENTS[event], key, 4), f"{event}.{key} can leave checks pending")
        self.assertIsNone(branch_filter("pull_request"))


if __name__ == "__main__":
    unittest.main()
