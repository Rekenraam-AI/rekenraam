#!/usr/bin/env python3
"""Create #445 alternating probe inputs: CAPTURE_DIRECTORY PROPOSED_GO OUTPUT.

Use the capture archive produced by investment-read-preparation-probe.patch.
The three rotation directories under OUTPUT must be new. This copies only
synthetic SQLite snapshots, never owner data.
"""
import json
from pathlib import Path
import shutil
import sys


def main():
    captured, proposed, output = map(Path, sys.argv[1:])
    text = proposed.read_text()
    sql = text.split("transfers, err := reader.QueryContext(ctx, `", 1)[1].split("`", 1)[0]
    output.mkdir(parents=True, exist_ok=True)
    if any((output / f"accepted-query-{n}").exists() for n in range(1, 4)):
        raise SystemExit("Use fresh rotation directories")
    for number in range(1, 4):
        directory = output / f"accepted-query-{number}"
        directory.mkdir()
        for path in sorted(captured.glob("*-queries.json")):
            rows = []
            for row in json.loads(path.read_text()):
                if "FROM investment_transfer_facts f" not in row["sql"] or (
                        "COALESCE(revision.source_lot_id, x.source_lot_id)" not in row["sql"]):
                    continue
                base = dict(row, caller=row["caller"] + ":base")
                head = dict(row, caller=row["caller"] + ":head", sql=sql)
                rows.extend((head, base) if number == 2 else (base, head))
            (directory / path.name).write_text(json.dumps(rows, indent=2) + "\n")
            database = path.with_name(path.name.replace("-queries.json", ".sqlite"))
            shutil.copy2(database, directory / database.name)


if __name__ == "__main__":
    main()
