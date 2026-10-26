"""Inventory native sqlite_schema captures against named tests.

Usage: python3 schema-baseline-inventory.py ROOT BASE_JSON HEAD_JSON BASE_DB OUTPUT
Captures must come from the native probe; Python SQLite only reads index/FK
metadata here. Text matches are discovery pointers, never branch-coverage proof.
"""
from pathlib import Path
import collections, hashlib, json, re, sqlite3, sys
root, base_path, head_path, database_path, output = map(Path, sys.argv[1:])
old = json.loads(base_path.read_text())
new = {o['name']: o for o in json.loads(head_path.read_text())}
database = sqlite3.connect(database_path.resolve().as_uri() + '?mode=ro', uri=True)
tests = []
for path in sorted((root / 'backend').rglob('*_test.go')):
    source = path.read_text()
    for match in re.finditer(r'^func (Test\w+)\(', source, re.M):
        end = source.find('\n}', match.start()) + 2
        tests.append((str(path.relative_to(root)), match[1], source[match.start():end]))
by_test = {name: (path, body) for path, name, body in tests}
manual = {
    'transaction_tags': ['TestBaselineTagLinksRefuseMissingOrArchivedTags', 'TestTransactionTagReplacementSemantics'],
    'posting_tags': ['TestBaselineTagLinksRefuseMissingOrArchivedTags', 'TestPostingResponseContractEmptyAndNonemptyTags'],
    'pricing_policies': ['TestPricingPolicy_GetAndSave_HTTP'],
    'pricing_refresh_state': ['TestFXCoverageBackfillsHistoricalPublicationDatesIdempotently'],
    'dividend_defaults': ['TestListAndSaveDividendDefaults', 'TestDividendDefaults_ListAndSave_HTTP'],
    'import_profiles': ['TestCSVProfileCreateListAndUploadAcceptance', 'TestImportProfileUpdateAndDeletePreserveHistoricalBatch'],
    'investment_capital_return_entitlements': ['TestCapitalReturnExplicitQuantitiesCapOnlyEntitledBasis'],
    'latest_investment_capital_return_revisions': ['TestBackdatedBuyReallocatesCapitalReturnAcrossEntitledLots'],
    'latest_investment_basis_resolution_provenance': ['TestProvenanceCorrectionImmutableHistoryExportsAndSurvivesRestore'],
    'commodity_versions_no_delete': ['TestUnusedInstrumentCleanupTreatsSecurityAndCryptoEqually', 'TestReferencedInstrumentCleanupRefusesBothKindsAtomically'],
    'investment_instrument_versions_no_delete': ['TestPostedInstrumentCleanupRefusesBothKindsBeforeVersionRemoval'],
    'account_versions_no_delete': ['TestCategoryDeleteRejectsCategoryUsedByPosting'],
    'goose_db_version': ['TestMigrateAppliesEmbeddedMigrations', 'TestMigrateResumesEmptyGooseHistory'],
    'sqlite_sequence': ['TestMigrateAppliesEmbeddedMigrations'],
}
removed_dormant = {'instrument_source_links', 'instrument_search_cache'}
removed_views = {o['name'] for o in old if o['type'] == 'view' and o['name'] not in new}
records = []
def pointer(name):
    path, _ = by_test[name]
    return {'file': path, 'test': name, 'basis': 'reviewed owner: workflow/read or direct refusal; inspect assertions for the individual predicate'}
def quote(text):
    return '"' + text.replace('"', '""') + '"'
for obj in old:
    name, table, typ, sql = obj['name'], obj['table'], obj['type'], obj['sql']
    messages = re.findall(r"RAISE\(ABORT,\s*'([^']+)'", sql, re.I)
    if typ == 'trigger':
        purpose = '; '.join(messages) if messages else ('Maintain FTS5 search index' if name == 'transaction_search_insert' else 'Enqueue/coalesce durable FX coverage on posted demand or first currency activation')
    elif typ == 'index':
        purpose = 'PRIMARY KEY/UNIQUE implementation owned by parent table' if not sql else 'Declared uniqueness/lookup/order access path; inspect SQL and index_xinfo'
    elif typ == 'view':
        purpose = 'Composed read/correction/validity semantics: ' + name
    else:
        purpose = 'Stored facts/state with column, CHECK, key and FK contracts in the exact SQL'
    ranked = []
    for file, test, body in tests:
        if table not in body or not re.search(r'\b' + re.escape(table) + r'\b', body):
            continue
        exact = sum(message in body for message in messages)
        score = 100 * exact + 20 * (name in body) + 3 * ('ErrorContains' in body) + 2 * ('immutable' in test.lower()) + min(5, body.count(table))
        if any(term in test for term in ['SeededBaseline', 'ExpectedSchema', 'AppliesEmbedded', 'DownRemoves']):
            score -= 200
        ranked.append((score, file, test, exact))
    ranked.sort(key=lambda row: (-row[0], row[1], row[2]))
    evidence = [{'file': file, 'test': test, 'basis': 'exact refusal text match (not complete branch coverage)' if exact else 'table/reference discovery pointer, not independent predicate proof'} for score, file, test, exact in ranked[:3]]
    owners = manual.get(name, manual.get(table, []))
    if name.startswith('transaction_search') or table.startswith('transaction_search'):
        owners = ['TestSQLiteIntegrityOnAReusedReaderAfterSearchWrites', 'TestRepeatedSelfCheckRunsPassAfterNewEntries']
    if typ == 'index' and name not in new and table not in removed_dormant:
        owners = ['TestBaselineIndexesKeepOrderedAndForeignKeyLookups']
    if name in removed_views:
        owners = ['TestCurrentVersionViewsUseEffectiveDateBeforeVersionSequence']
    if owners:
        evidence = [pointer(test) for test in owners] + [e for e in evidence if e['test'] not in owners][:1]
    if table in removed_dormant:
        evidence = [{'file': 'docs/reviews/schema-baseline-review-2026-10-26.md', 'basis': 'No runtime producer/reader or fixture; retired dormant storage. Exact native diff establishes absence. No invented behavioral test owner.'}]
    action = 'removed' if name not in new else ('unchanged' if sql == new[name]['sql'] else 'changed')
    record = {'type': typ, 'name': name, 'table': table, 'purpose': purpose, 'action': action,
              'old_rowid': obj['rowid'], 'new_rowid': new[name]['rowid'] if name in new else None,
              'old_sql': sql, 'old_sha256': hashlib.sha256(sql.encode()).hexdigest(),
              'new_sha256': hashlib.sha256(new[name]['sql'].encode()).hexdigest() if name in new else None,
              'test_evidence': evidence}
    if action == 'changed':
        record['new_sql'] = new[name]['sql']
    if typ == 'index':
        record['index_xinfo'] = [list(row) for row in database.execute('PRAGMA index_xinfo(' + quote(name) + ')')]
        record['metadata_note'] = 'seqno,cid,name,desc,collation,key; compare equality prefix, direction, predicate and uniqueness before dropping'
    if typ == 'table':
        record['foreign_key_list'] = [list(row) for row in database.execute('PRAGMA foreign_key_list(' + quote(name) + ')')]
        record['index_list'] = [list(row) for row in database.execute('PRAGMA index_list(' + quote(name) + ')')]
        record['constraint_evidence_limit'] = 'Exact stored SQL enumerates all CHECK/NULL/key/REFERENCES predicates. Owner pointers do not prove every invalid branch; unmodified predicates remain protected by original DDL and broader suites.'
    if name == 'account_versions_no_delete':
        record['review_finding'] = 'Custom-category exception permits used version deletion at SQL level; API refuses. See rollback probe and bounded follow-up in review.'
    records.append(record)
result = {'base': '9b414c08', 'baseline_identity': 20261026,
          'method': 'Every native sqlite_schema object, including implicit indexes and FTS/Goose internals. Exact stored SQL and rowids; constraint/FK/index metadata; named test ownership and evidence limits. No text match is claimed as exhaustive guard coverage.',
          'base_counts': dict(collections.Counter(o['type'] for o in old)),
          'head_counts': dict(collections.Counter(o['type'] for o in new.values())),
          'actions': dict(collections.Counter(r['action'] for r in records)),
          'added_objects': sorted(set(new) - {o['name'] for o in old}),
          'trigger_relative_order_equal': [o['name'] for o in old if o['type'] == 'trigger'] == [o['name'] for o in sorted(new.values(), key=lambda o:o['rowid']) if o['type'] == 'trigger'],
          'objects': records}
output.write_text(json.dumps(result, indent=2) + '\n')
print('actions', result['actions'], 'counts', result['base_counts'], result['head_counts'])
print('NO POINTER', [(r['type'], r['name']) for r in records if not r['test_evidence']])
