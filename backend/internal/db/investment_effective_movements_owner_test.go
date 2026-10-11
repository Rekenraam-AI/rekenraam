package db

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// #189: committed effective lot movements have one owner,
// investment_effective_movements.go. Folding effective_investment_lot_events
// and adding revisions back by hand is how a revised cover was forgotten
// (#188), so a new reader of those views must either use the owner or join
// this allow-list with a reason it is outside the owner's contract.
var effectiveHistoryReaders = map[string]string{
	"investment_position_side.go:requirePositionSideAvailableTx": "activity dates of effective operations, which revisions never change",

	"investment_capital_return.go:effectiveCapitalReturnEffectsQuery":                "one return of capital's current effects (per-subject terms)",
	"investment_split.go:effectiveSplitEffectsQuery":                                 "one split's current effects (per-subject terms)",
	"investment_cash_stock_merger_correction.go:CashStockMergerTermsByTransactionID": "one merger's current cash result (per-subject terms)",
	"investment_gain_impact.go:investmentGainSnapshotTx":                             "the gain-impact snapshot of disposal decisions",

	"investment_replay_intents.go:investmentReplayIntentsQuery":        "replay inputs keep original elections",
	"investment_replay_propagation.go:appendTransferLinkRevisionTx":    "replay appends the next link revision",
	"investment_replay_revisions.go:persistInvestmentReplayPositionTx": "replay appends the next disposal revision",
	"self_check_replay.go:storedInvestmentProjectionQuery":             "replay equivalence compares stored revisions with replay",
}

var effectiveHistoryView = regexp.MustCompile(`effective_investment_lot_events|latest_investment_[a-z_]+_revisions`)

func TestEffectiveHistoryReadersUseTheOwner(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, dir := range []string{".", "../app"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, file, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			base := filepath.Base(file)
			for _, decl := range parsed.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				reads := false
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING &&
						effectiveHistoryView.MatchString(literal.Value) {
						reads = true
					}
					return !reads
				})
				if !reads || base == "investment_effective_movements.go" {
					continue
				}
				key := base + ":" + fn.Name.Name
				seen[key] = true
				if _, allowed := effectiveHistoryReaders[key]; !allowed {
					t.Errorf("%s reads effective lot history directly: use EffectiveLotMovements, or allow-list it here with why it is outside the owner's contract",
						fset.Position(fn.Pos()))
				}
			}
		}
	}
	for key := range effectiveHistoryReaders {
		if !seen[key] {
			t.Errorf("allow-listed reader %s no longer reads effective lot history: remove it from the list", key)
		}
	}
}
