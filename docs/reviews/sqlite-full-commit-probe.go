// Standalone diagnostic for the 2026-10-12 durability review.
// Run from the repository root with ./scripts/go-backend.sh run ../docs/reviews/sqlite-full-commit-probe.go.
// Uses the same upstream native implementation; NORMAL is a diagnostic comparison only.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	ctx := context.Background()
	root, err := os.MkdirTemp("", "sqlite-commit-probe-")
	check(err)
	defer os.RemoveAll(root)
	reports := []map[string]any{}
	for round := 1; round <= 3; round++ {
		modes := []string{"NORMAL", "FULL"}
		if round%2 == 0 {
			modes = []string{"FULL", "NORMAL"}
		}
		for _, mode := range modes {
			file := filepath.Join(root, fmt.Sprintf("%s-%d.sqlite", mode, round))
			uri := (&url.URL{Scheme: "file", Path: file}).String() + "?_journal_mode=WAL&_synchronous=" + mode + "&_foreign_keys=1&_busy_timeout=5000&_txlock=deferred&_stmt_cache_size=0"
			db, err := sql.Open("sqlite3", uri)
			check(err)
			db.SetMaxOpenConns(1)
			_, err = db.ExecContext(ctx, `CREATE TABLE entries(id INTEGER PRIMARY KEY, amount TEXT NOT NULL); CREATE TABLE postings(entry_id INTEGER REFERENCES entries(id),amount TEXT NOT NULL); CREATE TABLE audit_events(entry_id INTEGER REFERENCES entries(id)); PRAGMA wal_autocheckpoint=1000`)
			check(err)
			var sync int
			check(db.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&sync))
			times := make([]float64, 500)
			totalStart := time.Now()
			for i := range times {
				start := time.Now()
				tx, err := db.BeginTx(ctx, nil)
				check(err)
				result, err := tx.ExecContext(ctx, "INSERT INTO entries(amount) VALUES(?)", "12345")
				check(err)
				id, err := result.LastInsertId()
				check(err)
				_, err = tx.ExecContext(ctx, "INSERT INTO postings VALUES(?,?),(?,?)", id, "12345", id, "-12345")
				check(err)
				_, err = tx.ExecContext(ctx, "INSERT INTO audit_events VALUES(?)", id)
				check(err)
				check(tx.Commit())
				times[i] = float64(time.Since(start)) / float64(time.Millisecond)
			}
			elapsed := time.Since(totalStart).Seconds()
			sort.Float64s(times)
			reports = append(reports, map[string]any{"round": round, "mode": mode, "effective_synchronous": sync, "commits": 500, "elapsed_seconds": elapsed, "median_transaction_ms": (times[249] + times[250]) / 2, "p95_transaction_ms": times[474], "max_transaction_ms": times[499]})
			check(db.Close())
		}
	}
	output, err := json.MarshalIndent(map[string]any{"description": "Isolated native SQLite WAL commit probe; 500 transactions with TEXT values, two balanced postings and one audit row. Includes preparation, writes and commit. Separate upstream driver connection permits diagnostic NORMAL comparison; shipped app always uses FULL. Not a complete application workflow or production latency guarantee.", "runs": reports}, "", "  ")
	check(err)
	fmt.Println(string(output))
}
