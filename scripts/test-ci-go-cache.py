#!/usr/bin/env python3
"""Exercise the real native cache-key script without contacting Actions storage."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


def configuration(workload="race", source="source-a", **overrides):
    with tempfile.TemporaryDirectory() as directory:
        paths = {name: str(Path(directory) / name) for name in (
            "GITHUB_OUTPUT", "GITHUB_ENV", "GITHUB_STEP_SUMMARY"
        )}
        env = dict(os.environ, **paths, GO_CACHE_WORKLOAD=workload,
                   GO_CACHE_SOURCE=source)
        env.update(overrides)
        subprocess.run([str(ROOT / "scripts/ci-go-cache.sh")], cwd=ROOT,
                       env=env, check=True, capture_output=True, text=True)
        outputs = dict(line.split("=", 1) for line in Path(paths["GITHUB_OUTPUT"]).read_text().splitlines())
        exported = dict(line.split("=", 1) for line in Path(paths["GITHUB_ENV"]).read_text().splitlines())
        return outputs, exported


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


if __name__ == "__main__":
    unittest.main()
