"""#402: summarise `go test -json` runs: wall, leaves, leaf work, chain-pattern leaves.

Usage: race-suite-cost-leaves.py run.json [run2.json ...] [--top N] [--pattern REGEX] [--list]
Leaf = test/subtest with no subtests (same definition as the slice-6 artifact).
"""
import json, re, sys

args = sys.argv[1:]
top = 0
pattern = r"(?i)migrat|downgrade|restore|backup|bundle"
listing = False
files = []
i = 0
while i < len(args):
    if args[i] == "--top":
        top = int(args[i + 1]); i += 2
    elif args[i] == "--pattern":
        pattern = args[i + 1]; i += 2
    elif args[i] == "--list":
        listing = True; i += 1
    else:
        files.append(args[i]); i += 1

for path in files:
    elapsed, fails, wall = {}, [], None
    for line in open(path):
        try:
            e = json.loads(line)
        except ValueError:
            continue
        test = e.get("Test")
        if e.get("Action") in ("pass", "fail", "skip"):
            if test:
                elapsed[test] = e.get("Elapsed", 0.0)
                if e["Action"] == "fail":
                    fails.append(test)
            else:
                wall = e.get("Elapsed")
                if e["Action"] == "fail":
                    fails.append("<package>")
    names = set(elapsed)
    leaves = {n: v for n, v in elapsed.items() if not any(m.startswith(n + "/") for m in names)}
    chain = {n: v for n, v in leaves.items() if re.search(pattern, n)}
    work = sum(leaves.values())
    print(f"{path}: wall={wall}s leaves={len(leaves)} work={work:.1f}s par={work / wall if wall else 0:.2f} "
          f"chain_leaves={len(chain)} chain_work={sum(chain.values()):.1f}s fails={fails[:5]}")
    if top:
        for n, v in sorted(leaves.items(), key=lambda kv: -kv[1])[:top]:
            print(f"  {v:7.2f}  {n}")
    if listing:
        for n, v in sorted(chain.items(), key=lambda kv: -kv[1]):
            print(f"  chain {v:7.2f}  {n}")
