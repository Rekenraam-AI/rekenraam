#!/usr/bin/env python3
"""Exercise the real native cache-key script without contacting Actions storage."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


def configuration(workload="race", source="source-a", **overrides):
    """Runs the script; source=None lets it fingerprint the repository itself."""
    with tempfile.TemporaryDirectory() as directory:
        paths = {name: str(Path(directory) / name) for name in (
            "GITHUB_OUTPUT", "GITHUB_ENV", "GITHUB_STEP_SUMMARY"
        )}
        env = dict(os.environ, **paths, GO_CACHE_WORKLOAD=workload)
        env.pop("GO_CACHE_SOURCE", None)
        if source is not None:
            env["GO_CACHE_SOURCE"] = source
        env.update(overrides)
        subprocess.run([str(ROOT / "scripts/ci-go-cache.sh")], cwd=ROOT,
                       env=env, check=True, capture_output=True, text=True)
        outputs = dict(line.split("=", 1) for line in Path(paths["GITHUB_OUTPUT"]).read_text().splitlines())
        exported = dict(line.split("=", 1) for line in Path(paths["GITHUB_ENV"]).read_text().splitlines())
        return outputs, exported


def git(repo, *args):
    subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True)


def commit(repo, files):
    """Writes files and commits them; returns the script's key for that commit."""
    for name, content in files.items():
        path = Path(repo) / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
    git(repo, "add", "-A")
    git(repo, "-c", "user.name=t", "-c", "user.email=t@example.invalid",
        "commit", "--quiet", "--allow-empty", "-m", "change")
    return configuration(source=None, GO_CACHE_SOURCE_REPO=str(repo))[0]["key"]


class GoCacheConfiguration(unittest.TestCase):
    def test_source_refresh_and_stable_repeated_run(self):
        first, exported = configuration()
        repeated, _ = configuration()
        changed, _ = configuration(source="source-b")
        self.assertEqual(first, repeated)
        self.assertNotEqual(first["key"], changed["key"])
        self.assertEqual(first["workload-prefix"], changed["workload-prefix"])
        self.assertTrue(changed["key"].startswith(first["workload-prefix"]))
        self.assertEqual(exported["GOCACHE"], first["gocache"])
        self.assertEqual(exported["GOMODCACHE"], first["gomodcache"])
        self.assertEqual(exported["GO_PHASE_TIMINGS"], "1")

    def test_writers_have_distinct_keys_with_compatible_fallbacks(self):
        outputs = [configuration(workload=workload)[0] for workload in (
            "race", "coverage", "build", "e2e-app", "e2e-audits"
        )]
        self.assertEqual(len({output["key"] for output in outputs}), 5)
        self.assertEqual(len({output["dependency-prefix"] for output in outputs}), 1)
        self.assertEqual(len({output["native-prefix"] for output in outputs}), 1)

    def test_native_configuration_is_a_hard_restore_boundary(self):
        original, _ = configuration()
        changed, _ = configuration(CGO_CFLAGS="-O0 -g")
        for prefix in ("native-prefix", "dependency-prefix", "workload-prefix"):
            self.assertNotEqual(original[prefix], changed[prefix])
            self.assertFalse(changed["key"].startswith(original[prefix]))


    def test_source_fingerprint_follows_exactly_the_listed_inputs(self):
        # Git object IDs replace hashFiles (#415): a listed input's content,
        # name or mode changes the key; anything else, uncommitted, does not.
        with tempfile.TemporaryDirectory() as repo:
            git(repo, "init", "--quiet")
            base = commit(repo, {"backend/main.go": "a", "frontend/app.ts": "a",
                                 "pnpm-lock.yaml": "a", "docs/readme.md": "a"})
            self.assertRegex(base, r"-[0-9a-f]{64}$")
            self.assertEqual(commit(repo, {}), base)
            self.assertEqual(commit(repo, {"docs/readme.md": "b"}), base)
            (Path(repo) / "backend/main.go").write_text("uncommitted")
            self.assertEqual(configuration(source=None, GO_CACHE_SOURCE_REPO=repo)[0]["key"], base)
            git(repo, "checkout", "--quiet", "--", "backend/main.go")
            changed = {commit(repo, {"backend/main.go": "b"})}
            changed.add(commit(repo, {"pnpm-lock.yaml": "b"}))
            changed.add(commit(repo, {"api/openapi.yaml": "new"}))
            changed.add(commit(repo, {".github/workflows/ci.yml": "new"}))
            git(repo, "mv", "frontend/app.ts", "frontend/main.ts")
            changed.add(commit(repo, {}))
            self.assertNotIn(base, changed)
            self.assertEqual(len(changed), 5)

    def test_this_checkout_has_a_stable_fingerprint(self):
        first, _ = configuration(source=None)
        repeated, _ = configuration(source=None)
        self.assertEqual(first, repeated)
        self.assertRegex(first["key"], r"-race-[0-9a-f]{64}$")

    def test_missing_fingerprint_is_a_cache_miss_not_a_failure(self):
        with tempfile.TemporaryDirectory() as not_a_repo:
            outputs, _ = configuration(source=None, GO_CACHE_SOURCE_REPO=not_a_repo,
                                       GITHUB_RUN_ID="41", GITHUB_RUN_ATTEMPT="2")
        normal, _ = configuration(source=None)
        self.assertEqual(outputs["key"], outputs["workload-prefix"] + "unavailable-41-2")
        self.assertEqual(outputs["workload-prefix"], normal["workload-prefix"])


if __name__ == "__main__":
    unittest.main()
