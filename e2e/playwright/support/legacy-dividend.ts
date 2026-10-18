import { execFileSync } from 'node:child_process';
import process from 'node:process';

/**
 * Frozen pre-#242 source linkage: the payment itself uses the real API.
 * Only the disposable e2e database gets an old ordinary-cash import marker;
 * no production endpoint is added to manufacture legacy financial records.
 * Callers skip when E2E_BASE_URL points at an external app.
 */
export function linkLegacyCashDividend(rowID: number, transactionID: number, accountID: number): void {
  // The calling worker's own database, published by fixtures.ts.
  const databasePath = process.env.E2E_DATABASE_PATH;
  if (!databasePath) throw new Error('E2E_DATABASE_PATH is unset: legacy fixtures need a self-managed e2e app');
  execFileSync('python3', ['-c', `
import sqlite3, sys
path, row_id, transaction_id, account_id = sys.argv[1:]
with sqlite3.connect(path, timeout=10) as db:
    db.execute('PRAGMA foreign_keys=ON')
    db.execute('PRAGMA synchronous=FULL')
    row = db.execute("SELECT s.book_id,s.dedupe_fingerprint,s.batch_id FROM import_staged_rows s JOIN import_batches b ON b.id=s.batch_id WHERE s.id=? AND s.commit_status='pending' AND b.source_kind='trading212' AND json_extract(s.raw_json,'$.kind')='trading212_dividend'", (row_id,)).fetchone()
    assert row is not None
    assert db.execute("SELECT transaction_kind FROM current_transaction_versions WHERE transaction_id=?", (transaction_id,)).fetchone()[0] == 'ordinary'
    identity = db.execute("INSERT INTO import_commit_identities(book_id,dedupe_fingerprint,source_fingerprint,source_kind,account_id,created_at) VALUES (?,?,?,'trading212',?,datetime('now'))", (row[0], row[1], row[1], account_id)).lastrowid
    db.execute('INSERT INTO import_commit_identity_effects(identity_id,effect_seq,transaction_id) VALUES (?,1,?)', (identity, transaction_id))
    db.execute("UPDATE import_staged_rows SET commit_status='committed',committed_identity_id=?,committed_transaction_id=? WHERE id=?", (identity, transaction_id, row_id))
    db.execute("UPDATE import_batches SET status='committed' WHERE id=?", (row[2],))
`, databasePath, String(rowID), String(transactionID), String(accountID)]);
}
