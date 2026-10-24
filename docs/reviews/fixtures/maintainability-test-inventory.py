#!/usr/bin/env python3
"""Reproduce #284 slice 6 test-ownership inventories at two fixed Git refs.

Extracts each ref with `git archive` (never the working tree), runs the
retained Go AST tools, and writes deterministic CSV/JSON outputs:

  reconciliation.csv   runtime functions added/removed/changed between refs,
                       flagged when the slice 3 ownership matrix names them
  fixtures.csv         consumer counts for shared Go fixture constructors
  raw-sql.json         raw SQL writes in Go tests, per table, at both refs
  assertions.csv       provenance of every testify equality assertion (head)
  financial-differential.csv
                       production-derived financial expectations, with the
                       count of literal anchors in the same test function
  e2e-classes.csv      per-spec assertion/fixture classes (lexical)
  e2e-counts.csv       exact-count assertions with a scope classification
  provenance.json      refs, tool hashes and headline counts

The classifications are lexical/AST navigation aids, not proofs. Every
category the report relies on was triaged by hand; see the report.
"""

import argparse
import csv
import hashlib
import json
import pathlib
import re
import subprocess
import sys
from collections import Counter, defaultdict

HERE = pathlib.Path(__file__).resolve().parent
REPO = HERE.parents[2]
MATRIX = REPO / 'docs/reviews/maintainability-invariants-2026-10-21.md'
TOOLS = {
    'hashes': HERE / 'maintainability-test-function-hashes.go',
    'assertions': HERE / 'maintainability-test-assertions.go',
}
FIXTURE_CONSTRUCTORS = [
    'newInvestmentsTestFixture', 'newInvestmentsTestFixtureWithOptions', 'newSetupTestHandler',
    'newSetupTestHandlerWithOptions', 'newSetupTestHandlerWithInvestment', 'newTestInstanceOnDatabase',
    'newInvestTestFixture', 'newPlainImportTestFixture', 'newSelfCheckHarness', 'openConnectionTestDatabase',
    'openTestDatabase', 'newRecurringTestDatabase', 'createInvestmentTestAccount', 'createPostedBuyLot',
    'seedTestSecurityCommodity', 'createHoldingAccountForSession', 'createInstrumentForSession',
    'createAccountForSession', 'seedReferenceBook', 'seedLargeBook',
]
MONEY = re.compile(r'(?i)(value|scale|basis|gain|proceeds|quantity|amount|total|balance|price|rate|cost|units|lots?\b|market|withholding|income)')
ANCHOR = re.compile(r'\b(require|assert)[A-Z]\w*\(t,\s*-?\d|\b(require|assert)[A-Z]\w*\(t,\s*"-?\d|ScaledIntFromInt64\(-?\d'
                    r'|\.Equal\(t,\s*(int64\()?-?\d|\.Equal\(t,\s*"-?\d')
E2E_CLASSES = {
    'route_landing': r'toHaveURL\(|toHaveTitle\(|getByRole\(\s*[\'"]heading',
    'accessibility': r'expectNoA11y|expectNoAccessibilityViolations|AxeBuilder|withRenderedContrast',
    'keyboard_focus': r'toBeFocused\(|keyboard\.press\(',
    'financial_literal': r'(toContainText|toHaveText|toHaveValue)\([^)]*[\'"`][^\'"`]*\d[\d.,\s  ]*\d[^\'"`]*[\'"`]'
                         r'|Intl\.NumberFormat\([^)]*\)\.format\(-?\d',
    'api_calls': r'apiJSON|apiExactText|exactJSON',
    'self_check': r'(?i)self[-_ ]?check',
    'exact_count': r'toHaveCount\(\s*\d',
    'unique_fixture_names': r'Date\.now\(\)|randomUUID|crypto\.random|testInfo\.(testId|workerIndex|parallelIndex)|test\.info\(\)',
    'direct_database_edit': r'legacy-dividend|node:sqlite|DatabaseSync',
    'route_stub': r'page\.route\(',
}
SCOPED = re.compile(r'filter\(|hasText|getByRole\([^)]*name|dialog|#[\w-]+|\[data-|getByTestId|row\(|within|section|form'
                    r'|getByLabel|nth\(|first\(\)|\bcard\b')


def run(*args, **kwargs):
    return subprocess.run(args, check=True, capture_output=True, text=True, **kwargs).stdout


def extract(ref, out):
    target = out / f'src-{ref[:8]}'
    if not target.exists():
        target.mkdir(parents=True)
        archive = subprocess.run(['git', '-C', str(REPO), 'archive', ref, 'backend', 'e2e', 'frontend/src'],
                                 check=True, capture_output=True).stdout
        subprocess.run(['tar', '-x', '-C', str(target)], input=archive, check=True)
    return target


def go_tool(name, out, *args):
    work = out / f'tool-{name}'
    work.mkdir(exist_ok=True)
    (work / 'main.go').write_text(TOOLS[name].read_text())
    if not (work / 'go.mod').exists():
        (work / 'go.mod').write_text(f'module audit{name}\n\ngo 1.27\n')
    return run('go', 'run', '.', *args, cwd=work)


def load_hashes(text):
    rows = defaultdict(list)
    for file, name, recv, lines, digest in csv.reader(text.splitlines()):
        rows[(recv, name)].append((file, digest))
    return rows


def matrix_names():
    names = set()
    for token in re.findall(r'`([A-Za-z_][A-Za-z0-9_./:]*)`', MATRIX.read_text()):
        leaf = re.split(r'[:/.]', token)[-1]
        if re.fullmatch(r'[A-Za-z]\w*', leaf):
            names.add(leaf)
    return names


def reconciliation(base_root, head_root, out):
    base = load_hashes(go_tool('hashes', out, str(base_root)))
    head = load_hashes(go_tool('hashes', out, str(head_root)))
    named = matrix_names()
    status = Counter()
    rows = []
    for key in sorted(set(base) | set(head)):
        b = sorted(d for _, d in base.get(key, []))
        h = sorted(d for _, d in head.get(key, []))
        state = 'unchanged' if b == h else 'removed' if not h else 'added' if not b else 'changed'
        status[state] += 1
        if state != 'unchanged':
            file = (head.get(key) or base.get(key))[0][0]
            rows.append([file, key[0], key[1], state, 'yes' if key[1] in named else 'no'])
    with (out / 'reconciliation.csv').open('w', newline='') as handle:
        writer = csv.writer(handle)
        writer.writerow(['file', 'receiver', 'function', 'state', 'named_in_slice3_matrix'])
        writer.writerows(rows)
    return dict(status), sum(len(v) for v in base.values()), sum(len(v) for v in head.values())


def fixture_consumers(root, out):
    tests = sorted(root.glob('backend/**/*_test.go'))
    sources = {p: p.read_text() for p in tests}
    with (out / 'fixtures.csv').open('w', newline='') as handle:
        writer = csv.writer(handle)
        writer.writerow(['constructor', 'call_sites', 'files'])
        for name in FIXTURE_CONSTRUCTORS:
            pattern = re.compile(r'(?<!func )\b' + name + r'\(')
            calls = {p: len(pattern.findall(s)) for p, s in sources.items()}
            writer.writerow([name, sum(calls.values()), sum(1 for n in calls.values() if n)])


def raw_sql(root):
    tables, kinds, files = Counter(), Counter(), set()
    for path in root.glob('backend/**/*_test.go'):
        for match in re.finditer(r'(?is)\b(INSERT\s+(?:OR\s+\w+\s+)?INTO|UPDATE|DELETE\s+FROM)\s+([a-z_][a-z0-9_]*)', path.read_text()):
            table = match.group(2)
            if table in {'set', 'or', 'the', 'a', 'it', 'its', 'this', 'that', 'to', 'of', 'on'}:
                continue
            tables[table] += 1
            kinds[match.group(1).split()[0].upper()] += 1
            files.add(str(path.relative_to(root)))
    return {'statements': sum(tables.values()), 'kinds': dict(sorted(kinds.items())), 'files': len(files),
            'tables': dict(sorted(tables.items(), key=lambda item: (-item[1], item[0])))}


def assertions(root, out):
    text = go_tool('assertions', out, str(root / 'backend'))
    (out / 'assertions.csv').write_text(text)
    rows = list(csv.DictReader(text.splitlines()))
    by_function = defaultdict(list)
    for row in rows:
        by_function[(row['file'], row['func'])].append(row)
    flagged = []
    for (file, function), items in sorted(by_function.items()):
        derived = [r for r in items if r['expected_source'] == 'production' and MONEY.search(r['expected_expr'])]
        if not derived:
            continue
        source = (root / 'backend' / file).read_text()
        body = re.search(r'^func (\([^)]*\) )?' + re.escape(function) + r'\b.*?^}', source, re.S | re.M)
        anchors = len(ANCHOR.findall(body.group(0))) if body else 0
        flagged.append([file, function, len(derived), anchors, ' '.join(sorted({c for r in derived for c in r['expected_calls'].split()}))])
    with (out / 'financial-differential.csv').open('w', newline='') as handle:
        writer = csv.writer(handle)
        writer.writerow(['file', 'test_function', 'production_derived_financial_expectations', 'literal_anchors', 'expected_calls'])
        writer.writerows(flagged)
    pairs = Counter(f"{r['expected_source']}->{r['actual_source']}" for r in rows)
    return {'equality_assertions': len(rows), 'by_provenance': dict(sorted(pairs.items())),
            'financial_differential_functions': len(flagged),
            'zero_anchor_functions': sum(1 for row in flagged if row[3] == 0)}


def e2e(root, out):
    specs = sorted(p for p in root.glob('e2e/**/*.spec.ts') if 'node_modules' not in p.parts)
    totals = Counter()
    with (out / 'e2e-classes.csv').open('w', newline='') as handle:
        writer = csv.writer(handle)
        writer.writerow(['spec', 'test_declarations', *E2E_CLASSES])
        for path in specs:
            source = path.read_text()
            counts = [len(re.findall(pattern, source)) for pattern in E2E_CLASSES.values()]
            for name, count in zip(E2E_CLASSES, counts):
                totals[name] += bool(count)
            writer.writerow([str(path.relative_to(root)),
                             len(re.findall(r'^\s*test(\.\w+)?\(\s*[\'"`]', source, re.M)), *counts])
    scope = Counter()
    with (out / 'e2e-counts.csv').open('w', newline='') as handle:
        writer = csv.writer(handle)
        writer.writerow(['spec', 'line', 'expected', 'scope', 'expression'])
        for path in specs:
            for number, line in enumerate(path.read_text().splitlines(), 1):
                if not re.search(r'toHaveCount\(\s*\d', line):
                    continue
                expected = 'zero' if re.search(r'toHaveCount\(\s*0\s*\)', line) else 'nonzero'
                kind = ('scoped-locator' if SCOPED.search(line)
                        else 'page-rows' if re.search(r"page\.locator\(['\"](tbody tr|tr|li)", line)
                        else 'named-variable')
                scope[(expected, kind)] += 1
                writer.writerow([str(path.relative_to(root)), number, expected, kind, line.strip()])
    return {'specs': len(specs), 'specs_with_class': dict(totals),
            'exact_counts': {f'{a}/{b}': n for (a, b), n in sorted(scope.items())}}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--base', required=True)
    parser.add_argument('--head', required=True)
    parser.add_argument('--output', required=True, type=pathlib.Path)
    args = parser.parse_args()
    out = args.output.resolve()
    out.mkdir(parents=True, exist_ok=True)
    base = run('git', '-C', str(REPO), 'rev-parse', args.base).strip()
    head = run('git', '-C', str(REPO), 'rev-parse', args.head).strip()
    base_root, head_root = extract(base, out), extract(head, out)
    status, base_count, head_count = reconciliation(base_root, head_root, out)
    fixture_consumers(head_root, out)
    sql = {'base': raw_sql(base_root), 'head': raw_sql(head_root)}
    (out / 'raw-sql.json').write_text(json.dumps(sql, indent=2) + '\n')
    provenance = {
        'base': base, 'head': head,
        'tools': {name: hashlib.sha256(path.read_bytes()).hexdigest() for name, path in sorted(TOOLS.items())},
        'runtime_functions': {'base': base_count, 'head': head_count, 'states': dict(sorted(status.items()))},
        'raw_sql_statements': {'base': sql['base']['statements'], 'head': sql['head']['statements']},
        'assertions': assertions(head_root, out),
        'e2e': e2e(head_root, out),
    }
    (out / 'provenance.json').write_text(json.dumps(provenance, indent=2, sort_keys=True) + '\n')
    json.dump(provenance, sys.stdout, indent=2, sort_keys=True)
    print()


if __name__ == '__main__':
    main()
