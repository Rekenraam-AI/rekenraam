package db

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// exactNumberRule is one coefficient column's legality rule (#192). Every
// rule also requires exact.Coefficient's canonical representation: digits,
// an optional leading '-', no leading zero, no '-0', at most maxDigits digits.
type exactNumberRule struct {
	legality  string // signed, signed_nonzero, nonnegative or positive
	maxDigits int
}

var (
	signedNumber        = exactNumberRule{"signed", 38}
	signedNonzeroNumber = exactNumberRule{"signed_nonzero", 38}
	nonnegativeNumber   = exactNumberRule{"nonnegative", 38}
	positiveNumber      = exactNumberRule{"positive", 38}
	// A basis fraction is at most 1 at scale 12 or less.
	positiveFraction    = exactNumberRule{"positive", 13}
	nonnegativeFraction = exactNumberRule{"nonnegative", 13}
)

// exactNumberColumnRules states the rule of every exact-number coefficient
// column. Quantities and basis amounts are nonnegative where a decision can
// dispose nothing: a cash-and-stock merger's cash decision and its
// allocations have quantity 0 (#181). Proceeds, journal quantities and lot
// event movements are signed; a split's revised delta is never zero.
var exactNumberColumnRules = map[string]exactNumberRule{
	"posting_versions.quantity_value":                                    signedNumber,
	"reconciliation_sessions.statement_balance_value":                    signedNumber,
	"reconciliation_sessions.starting_balance_value":                     signedNumber,
	"reconciliation_sessions.selected_balance_value":                     signedNumber,
	"reconciliation_sessions.difference_value":                           signedNumber,
	"reconciliation_session_postings.quantity_value":                     signedNumber,
	"reconciliation_checkpoints.statement_balance_value":                 signedNumber,
	"reconciliation_checkpoint_postings.quantity_value":                  signedNumber,
	"recurring_template_postings.quantity_value":                         signedNumber,
	"budget_targets.quantity_value":                                      nonnegativeNumber,
	"investment_lots.quantity_value":                                     positiveNumber,
	"investment_lots.cost_basis_value":                                   nonnegativeNumber,
	"investment_lot_state.remaining_quantity_value":                      nonnegativeNumber,
	"investment_lot_state.remaining_cost_basis_value":                    nonnegativeNumber,
	"investment_lot_events.quantity_value":                               signedNumber,
	"investment_lot_events.cost_basis_value":                             signedNumber,
	"investment_disposal_decisions.quantity_value":                       nonnegativeNumber,
	"investment_disposal_decisions.disposed_basis_value":                 nonnegativeNumber,
	"investment_disposal_decisions.proceeds_value":                       signedNumber,
	"investment_disposal_allocations.quantity_value":                     nonnegativeNumber,
	"investment_disposal_allocations.cost_basis_value":                   nonnegativeNumber,
	"investment_disposal_allocations.proceeds_value":                     signedNumber,
	"investment_disposal_clearing_allocations.proceeds_value":            signedNumber,
	"investment_disposal_revisions.disposed_basis_value":                 nonnegativeNumber,
	"investment_disposal_revision_allocations.quantity_value":            nonnegativeNumber,
	"investment_disposal_revision_allocations.cost_basis_value":          nonnegativeNumber,
	"investment_disposal_revision_allocations.proceeds_value":            signedNumber,
	"investment_operation_components.amount_value":                       signedNumber,
	"investment_transfer_facts.basis_fraction_value":                     positiveFraction,
	"investment_transfer_facts.cash_basis_fraction_value":                nonnegativeFraction,
	"investment_transfer_lot_links.quantity_value":                       positiveNumber,
	"investment_transfer_lot_links.carried_basis_value":                  nonnegativeNumber,
	"investment_transfer_link_revisions.carried_basis_value":             nonnegativeNumber,
	"investment_transfer_link_revision_depletions.quantity_value":        positiveNumber,
	"investment_transfer_link_revision_depletions.cost_basis_value":      nonnegativeNumber,
	"investment_basis_resolutions.quantity_value":                        positiveNumber,
	"investment_basis_resolutions.basis_value":                           nonnegativeNumber,
	"investment_split_revision_effects.quantity_delta_value":             signedNonzeroNumber,
	"investment_capital_return_facts.amount_value":                       positiveNumber,
	"investment_capital_return_effects.entitled_quantity_value":          positiveNumber,
	"investment_capital_return_effects.allocated_value":                  nonnegativeNumber,
	"investment_capital_return_effects.reduction_value":                  nonnegativeNumber,
	"investment_capital_return_effects.excess_value":                     nonnegativeNumber,
	"investment_capital_return_entitlements.quantity_value":              positiveNumber,
	"investment_capital_return_revision_effects.entitled_quantity_value": positiveNumber,
	"investment_capital_return_revision_effects.allocated_value":         nonnegativeNumber,
	"investment_capital_return_revision_effects.reduction_value":         nonnegativeNumber,
	"investment_capital_return_revision_effects.excess_value":            nonnegativeNumber,
}

type exactNumberColumn struct {
	table, column string
	nullable      bool
}

// exactNumberColumns finds every TEXT coefficient column: a *_value column
// beside an INTEGER *_scale column of the same prefix.
func exactNumberColumns(t *testing.T, database *sql.DB) []exactNumberColumn {
	t.Helper()
	tables, err := database.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	require.NoError(t, err)
	var names []string
	for tables.Next() {
		var name string
		require.NoError(t, tables.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, tables.Err())
	require.NoError(t, tables.Close())

	var columns []exactNumberColumn
	for _, table := range names {
		rows, err := database.Query(`SELECT name, type, "notnull" FROM pragma_table_info(?)`, table)
		require.NoError(t, err)
		types := map[string]string{}
		notNull := map[string]bool{}
		var order []string
		for rows.Next() {
			var name, kind string
			var required bool
			require.NoError(t, rows.Scan(&name, &kind, &required))
			types[name], notNull[name] = kind, required
			order = append(order, name)
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
		for _, name := range order {
			prefix, found := strings.CutSuffix(name, "_value")
			if found && types[name] == "TEXT" && types[prefix+"_scale"] == "INTEGER" {
				columns = append(columns, exactNumberColumn{table: table, column: name, nullable: !notNull[name]})
			}
		}
	}
	return columns
}

// columnCheckExpression is the column's own CHECK, read from the stored
// schema so the test exercises exactly what the baseline installed.
func columnCheckExpression(t *testing.T, database *sql.DB, table, column string) string {
	t.Helper()
	var definition string
	require.NoError(t, database.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&definition))
	location := regexp.MustCompile(`(?m)^\s+` + column + ` TEXT\b`).FindStringIndex(definition)
	require.NotNil(t, location, "%s.%s definition", table, column)
	rest := definition[location[1]:]
	start := strings.Index(rest, "CHECK (")
	require.False(t, start < 0 || strings.Contains(rest[:start], ",\n"), "%s.%s has no column CHECK", table, column)
	depth, quoted := 0, false
	for index := start + len("CHECK "); index < len(rest); index++ {
		switch rest[index] {
		case '\'':
			quoted = !quoted
		case '(':
			if !quoted {
				depth++
			}
		case ')':
			if !quoted {
				depth--
				if depth == 0 {
					return rest[start+len("CHECK (") : index]
				}
			}
		}
	}
	require.FailNow(t, "unbalanced CHECK", "%s.%s", table, column)
	return ""
}

func TestExactNumberColumnChecks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := openTestDatabase(t)
	require.NoError(t, Migrate(ctx, database))

	columns := exactNumberColumns(t, database)
	found := make([]string, 0, len(columns))
	for _, column := range columns {
		found = append(found, column.table+"."+column.column)
	}
	stated := make([]string, 0, len(exactNumberColumnRules))
	for key := range exactNumberColumnRules {
		stated = append(stated, key)
	}
	require.ElementsMatch(t, stated, found, "every coefficient column states one rule")

	digits := func(first byte, n int) string { return string(first) + strings.Repeat("9", n-1) }
	for _, column := range columns {
		t.Run(column.table+"."+column.column, func(t *testing.T) {
			rule := exactNumberColumnRules[column.table+"."+column.column]
			expression := columnCheckExpression(t, database, column.table, column.column)
			passes := func(value any) bool {
				var ok bool
				require.NoError(t, database.QueryRow(`SELECT (`+expression+`) IS NOT 0 FROM (SELECT ? AS `+column.column+`)`, value).Scan(&ok))
				return ok
			}
			widest := digits('9', rule.maxDigits)
			tooWide := digits('1', rule.maxDigits+1)
			signed := rule.legality == "signed" || rule.legality == "signed_nonzero"
			zero := rule.legality == "signed" || rule.legality == "nonnegative"
			expect := map[string]bool{
				"1": true, "10": true, "25": true, widest: true,
				"0":  zero,
				"-1": signed, "-" + widest: signed, "-10": signed,
				// Never canonical.
				"-0": false, "00": false, "007": false, "-007": false, "-00": false,
				"": false, "-": false, "--1": false, "+1": false, " 1": false, "1 ": false,
				"1.5": false, "1e5": false, "0x1": false, "abc": false, "1-": false,
				tooWide: false, "-" + tooWide: false,
			}
			for value, want := range expect {
				require.Equal(t, want, passes(value), "%s %s with %q", rule.legality, column.column, value)
			}
			if column.nullable {
				require.True(t, passes(nil), "a nullable coefficient's own CHECK leaves NULL to its pair CHECK")
			}
		})
	}
}
