package db

import (
	"context"
	"database/sql"
)

// statementPreparer is what a statement set prepares on: in practice the
// enclosing *sql.Tx.
type statementPreparer interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// statementExecQueryer is the subset of *sql.Tx that loop helpers use, so a
// caller can hand them either the transaction or its statement set.
type statementExecQueryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// statementSet compiles each distinct query at most once for the lifetime of
// one call that runs the same SQL in a loop (#404). The DSN keeps the native
// statement cache off, so every plain ExecContext/QueryContext recompiles its
// SQL — for an INSERT, together with every trigger on the target table — and
// statement compilation was a third of the application race suite's CPU.
//
// Statements are prepared lazily on first use, so a query a call never runs
// is never compiled. A set belongs to one goroutine and one transaction: close
// it before the transaction ends, and consume a query's rows before running
// the same query again.
type statementSet struct {
	preparer   statementPreparer
	statements map[string]*sql.Stmt
}

func newStatementSet(preparer statementPreparer) *statementSet {
	return &statementSet{preparer: preparer, statements: make(map[string]*sql.Stmt)}
}

func (s *statementSet) prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	if statement, ok := s.statements[query]; ok {
		return statement, nil
	}
	statement, err := s.preparer.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	s.statements[query] = statement
	return statement, nil
}

func (s *statementSet) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	statement, err := s.prepare(ctx, query)
	if err != nil {
		return nil, err
	}
	return statement.ExecContext(ctx, args...)
}

func (s *statementSet) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	statement, err := s.prepare(ctx, query)
	if err != nil {
		return nil, err
	}
	return statement.QueryContext(ctx, args...)
}

// QueryRowContext reports a compilation failure through Scan, as the
// unprepared call does, by letting the preparer run the query itself.
func (s *statementSet) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	statement, err := s.prepare(ctx, query)
	if err != nil {
		return s.preparer.QueryRowContext(ctx, query, args...)
	}
	return statement.QueryRowContext(ctx, args...)
}

// Close releases every statement the set compiled. The transaction would
// release them at commit or rollback; closing at the end of the owning call
// keeps their lifetime visible where they are used.
func (s *statementSet) Close() {
	for query, statement := range s.statements {
		_ = statement.Close()
		delete(s.statements, query)
	}
}

// rowsQueryer is the read seam the checkpoint selectors accept.
type rowsQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// reuseTxStatements gives a looping reader a statement set when it runs inside
// a transaction. A pool keeps per-call preparation: its statements are not
// bound to one connection or one transaction.
func reuseTxStatements(queryer rowsQueryer) (rowsQueryer, func()) {
	tx, ok := queryer.(*sql.Tx)
	if !ok {
		return queryer, func() {}
	}
	statements := newStatementSet(tx)
	return statements, statements.Close
}
