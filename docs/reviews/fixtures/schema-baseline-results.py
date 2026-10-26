"""Reduce #405 matched logs; RUN_DIR PROBE_DIR HOST_JSON OUTPUT.

Run only after all matched jobs finish. Raw logs remain outside the repository;
this emits durations, outcomes, leaf digests, case changes and schema probes.
"""
from pathlib import Path
import collections, hashlib, json, re, statistics, sys
runs, probes, host, output = map(Path, sys.argv[1:])
summary = json.loads((runs / 'matched-summary.json').read_text())
latest = {r['name']: r for r in summary['runs']}
result = {'base': '9b414c08', 'head_implementation': 'f4ee790fad60b01efb2262dc626efdb412c97b3b', 'baseline_identity': 20261026,
          'host_and_source': json.loads(host.read_text()),
          'configuration': 'go1.27.3, mattn/go-sqlite3 v1.14.52 / bundled SQLite 3.53.4; CGO, fts5/netgo/osusergo; FULL; race; statement cache 0; GOMAXPROCS unset; full gate -p 2 -timeout=5m; isolated prebuilt app -test.timeout=10m per #402',
          'commands': {'matched': 'schema-baseline-matched.py BASE HEAD OUTPUT (GOCACHE exported)',
                       'full_gate': 'GOFLAGS=-count=1 GO_PHASE_TIMINGS=1 scripts/test-backend.sh'},
          'application': [], 'full_gate': [], 'probe': {},
          'validation': {'frontend_unit_pass': 778, 'integrated_build': 'pass', 'release_seed_round_trip': 'pass, 272 statements, native constraints and source snapshots', 'browser_initial': '449 passed / 1 transient timeout in the 450-case smoke; route error sweep', 'browser_rerun': 'same case twice passed; complete audits group 12 passed', 'final_binary_bootstrap': '2 passed after startup-admission correction', 'negative_control': 'original schema: unused stock passes, unused crypto fails; both referenced and posted kinds refuse as expected', 'hosted_ci': 'not run; no push'},
          'excluded_development_attempts': [r for r in summary['runs'] if r['exit'] != 0],
          'measurement_limits': ['Single shared host, no confidence interval or hosted CI claim.', 'No competing test/build jobs during accepted runs. A one-off inventory scan overlapped part of original full gate 1; do not attribute its small timing differences solely to this change.', 'Schema probe uses five batches of 100 operations per stage; version stages are sequential, not a randomized benchmark.', 'Application test names change: retired obsolete transitions, moved final assertions and added controls are separately mapped. Suite-time differences do not isolate production schema cost.']}
leaf_sets = {}
for label in ['base', 'head']:
    for number in range(1, 4):
        name = f'app-{label}-{number}'
        run = latest[name]
        assert run['exit'] == 0, run
        terminal, actions, wall, package_action = {}, {}, None, None
        for line in (runs / (name + '.log')).read_text().splitlines():
            try:
                event = json.loads(line)
            except ValueError:
                continue
            if event.get('Action') not in ['pass', 'fail', 'skip']:
                continue
            if event.get('Test'):
                terminal[event['Test']] = event.get('Elapsed', 0)
                actions[event['Test']] = event['Action']
            else:
                wall, package_action = event.get('Elapsed'), event['Action']
        names = set(terminal)
        leaves = {n: value for n, value in terminal.items() if not any(other.startswith(n + '/') for other in names)}
        failures = [n for n in names if actions[n] == 'fail']
        assert package_action == 'pass' and not failures, (name, package_action, failures)
        digest = hashlib.sha256(('\n'.join(sorted(leaves)) + '\n').encode()).hexdigest()
        leaf_sets[label] = set(leaves)
        result['application'].append({'name': name, 'package_wall_s': wall, 'command_wall_s': run['wall_s'], 'leaves': len(leaves), 'leaf_work_s': round(sum(leaves.values()), 3), 'outcomes': dict(collections.Counter(actions[n] for n in leaves)), 'skipped_cases': sorted(n for n in leaves if actions[n] == 'skip'), 'leaf_case_name_sha256': digest, 'started_utc': run['started_utc']})
    for number in [1, 2]:
        name = f'full-{label}-{number}'
        run = latest[name]
        assert run['exit'] == 0, run
        log = (runs / (name + '.log')).read_text()
        packages = {}
        # Each package appears in compile-only and real execution phases; keep
        # its last occurrence, never the synthetic 0.001s compile success.
        for package, elapsed in re.findall(r'^ok\s+(rekenraam/\S+)\s+(\d+(?:\.\d+)?)s', log, re.M):
            packages[package] = float(elapsed)
        phases = {phase: {'wall_s': float(wall), 'user_s': float(user), 'sys_s': float(system)} for phase, wall, user, system in re.findall(r'Go phase ([\w-]+): wall=([\d.]+)s user=([\d.]+)s sys=([\d.]+)s', log)}
        result['full_gate'].append({'name': name, 'command_wall_s': run['wall_s'], 'package_wall_s': packages, 'phases': phases, 'started_utc': run['started_utc'], 'exit': run['exit']})
    rows = json.loads((probes / ('final-' + label + '-probe.json')).read_text())
    for row in rows:
        row['open_median_ms'] = statistics.median(row['OpenMS'])
        row['prepare_median_ms'] = statistics.median(row['PrepareMS']) if row['PrepareMS'] else None
    result['probe'][label] = rows
for label in ['base', 'head']:
    assert len({r['leaf_case_name_sha256'] for r in result['application'] if r['name'].startswith('app-' + label)}) == 1, label
result['leaf_digest_method'] = 'sha256 of sorted leaf names joined with newline and a trailing newline; leaves have no descendant subtest'
result['application_leaf_names_removed_or_renamed'] = sorted(leaf_sets['base'] - leaf_sets['head'])
result['application_leaf_names_added_or_renamed'] = sorted(leaf_sets['head'] - leaf_sets['base'])
result['application_median_s'] = {label: statistics.median(r['package_wall_s'] for r in result['application'] if r['name'].startswith('app-' + label)) for label in ['base', 'head']}
result['adr0016'] = {'single_app_investigate_210s': result['application_median_s']['head'] > 210,
                     'two_full_gate_package_runs_exceed_270s': [package for package in result['full_gate'][2]['package_wall_s'] if all(r['package_wall_s'].get(package, 0) > 270 for r in result['full_gate'] if r['name'].startswith('full-head-'))]}
output.write_text(json.dumps(result, indent=2) + '\n')
print(json.dumps({'medians': result['application_median_s'], 'thresholds': result['adr0016'], 'app_runs': result['application']}, indent=2))
