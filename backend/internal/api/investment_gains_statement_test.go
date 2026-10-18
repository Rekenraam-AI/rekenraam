package api

import (
	"context"
	"encoding/json"
	"github.com/pressly/goose/v3"
	"net/http"
	"rekenraam/backend/internal/app"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/migrations"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

// #231: the period statement is one composed read per period: opening and
// closing unrealized under operational_v1, realized and income in between,
// with absent figures null and never zero.
func TestInvestmentGainsStatementAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "STMT")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	// Bought 2 for 40.00 on 2026-02-01 (implied price 20.00); sold 1 for 50.00
	// on 2026-03-01 (implied price 50.00); a 5.00 dividend in the period.
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy",
		tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 4000), http.StatusCreated)
	sale := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 5000)
	sale.TransactionDate = "2026-03-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/sell", sale, http.StatusCreated)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/dividend", dividendRequest{
		TransactionDate: "2026-03-15", CashAccountID: f.cashAccount.ID, CashCommodityID: f.commodityID,
		IncomeAccountID: &f.incomeAccount.ID, AmountValue: 500, AmountScale: 2,
	}, http.StatusCreated)

	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		"/api/v1/investments/gains/statement?from=2026-03-01&to=2026-03-31", nil, http.StatusOK)
	var body investmentGainsStatementResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "operational_v1", body.Profile.Name)
	assert.Equal(t, "2026-03-31", body.Profile.ValuationAsOf)
	assert.Equal(t, "2026-03-01", body.From)
	assert.Equal(t, "2026-03-31", body.To)
	assert.NotEmpty(t, body.Currencies)
	require.Len(t, body.Rows, 1)
	row := body.Rows[0]
	assert.Equal(t, f.commodityID, row.CommodityID)
	amount := func(value *moneyCoefficient, scale *int) string {
		require.NotNil(t, value)
		require.NotNil(t, scale)
		// Figures keep the scale their exact arithmetic needed; compare values.
		return exact.ScaledIntFromInt64(int64(*value), *scale).Normalized().String()
	}
	equals := func(decimal string) string {
		parts := strings.SplitN(decimal, ".", 2)
		scale := 0
		if len(parts) == 2 {
			scale = len(parts[1])
		}
		coefficient, err := strconv.ParseInt(strings.Join(parts, ""), 10, 64)
		require.NoError(t, err)
		return exact.ScaledIntFromInt64(coefficient, scale).Normalized().String()
	}
	assert.Equal(t, "2026-02-28", row.Opening.ValuationAsOf)
	assert.Equal(t, equals("0.00"), amount(row.Opening.UnrealizedGainValue, row.Opening.UnrealizedGainScale), "priced at the 20.00 fill")
	assert.Equal(t, "2026-02-01", *row.Opening.LatestPriceDate)
	assert.Equal(t, equals("30.00"), amount(row.Closing.UnrealizedGainValue, row.Closing.UnrealizedGainScale), "1 × 50.00 − 20.00")
	assert.Equal(t, "2026-03-01", *row.Closing.EarliestPriceDate)
	assert.Equal(t, 1, row.Closing.StaleCount, "thirty days old at the period end")
	assert.Equal(t, equals("30.00"), amount(row.RealizedGainValue, row.RealizedGainScale), "50.00 − 20.00")
	assert.Equal(t, 1, row.DisposalCount)
	assert.Equal(t, equals("5.00"), amount(&row.IncomeValue, &row.IncomeScale))
	assert.Equal(t, 1, row.IncomeCount)
	assert.Zero(t, row.UnclassifiedDividends.PaymentCount, "a named dividend is income, not unclassified cash")
	assert.Equal(t, "0", row.UnclassifiedDividends.Cash.Value.String())
	assert.Equal(t, equals("30.00"), amount(row.UnrealizedChangeValue, row.UnrealizedChangeScale))
	assert.Equal(t, equals("65.00"), amount(row.TotalValue, row.TotalScale), "realized + income + change")

	for _, query := range []string{"", "?from=2026-03-01", "?from=2026-04-01&to=2026-03-31", "?from=2026-03-01&to=2099-01-01", "?from=2026-02-30&to=2026-03-31"} {
		res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains/statement"+query, nil, http.StatusBadRequest)
		var failure errorResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&failure))
		assert.Equal(t, "VALIDATION_FAILED", failure.Error.Code, query)
	}
}

// An absent figure is null on the wire with its reason and counts, never "0".
func TestInvestmentGainsStatementAPIAbsentFiguresAreNull(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "ABSENT")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	// Unknown basis and no price at the opening date; the sale's implied
	// price prices the remaining unit at the closing date.
	in := externalTransferInRequest{EffectiveOn: "2026-02-01", HoldingAccountID: holding.ID, BasisKnowledge: "unknown",
		CommodityID: instrument.CommodityID, QuantityValue: exact.New(2), CostCommodityID: f.commodityID}
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/transfers/external/in", in, http.StatusCreated)
	sale := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 6000)
	sale.TransactionDate = "2026-03-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/sell", sale, http.StatusCreated)

	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/return-of-capital", capitalReturnRequest{
		HoldingAccountID: holding.ID, CommodityID: instrument.CommodityID, CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID,
		EffectiveOn: "2026-03-15", PaymentOn: "2026-03-20", AmountValue: exact.New(1000), AmountScale: 2, SourceEvidence: json.RawMessage(`{"notice":"unknown238"}`),
	}, http.StatusCreated)
	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		"/api/v1/investments/gains/statement?from=2026-03-01&to=2026-03-31", nil, http.StatusOK)
	var raw struct {
		Rows []map[string]json.RawMessage `json:"rows"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&raw))
	require.Len(t, raw.Rows, 1)
	row := raw.Rows[0]
	var opening, closing map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(row["opening"], &opening))
	require.NoError(t, json.Unmarshal(row["closing"], &closing))
	assert.JSONEq(t, `null`, string(opening["unrealized_gain_value"]))
	assert.JSONEq(t, `"unpriced_and_unresolved"`, string(opening["unavailable_reason"]))
	assert.JSONEq(t, `1`, string(opening["unpriced_count"]))
	assert.JSONEq(t, `null`, string(opening["latest_price_date"]))
	assert.JSONEq(t, `null`, string(closing["unrealized_gain_value"]))
	assert.JSONEq(t, `"unresolved_basis"`, string(closing["unavailable_reason"]))
	assert.JSONEq(t, `"2026-03-01"`, string(closing["latest_price_date"]), "the price is stated even when basis is unknown")
	assert.JSONEq(t, `null`, string(row["realized_gain_value"]))
	assert.JSONEq(t, `1`, string(row["realized_unresolved_count"]))
	assert.JSONEq(t, `"0"`, string(row["income_value"]), "no dividends is a known zero")
	assert.JSONEq(t, `null`, string(row["unrealized_change_value"]))
	assert.JSONEq(t, `null`, string(row["total_value"]))
	var disclosure map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(row["capital_return"], &disclosure))
	assert.JSONEq(t, `null`, string(disclosure["reduction"]))
	assert.JSONEq(t, `null`, string(disclosure["excess"]))
	assert.JSONEq(t, `1`, string(disclosure["unknown_lots"]))
	assert.JSONEq(t, `1`, string(disclosure["unresolved_count"]))
}

func TestInvestmentGainsStatementDisclosesExcludedCapitalReturnAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "ROC238")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 700)
	buy.TransactionDate = "2026-05-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/return-of-capital", capitalReturnRequest{
		HoldingAccountID: holding.ID, CommodityID: instrument.CommodityID, CashAccountID: f.cashAccount.ID,
		CurrencyID: f.commodityID, EffectiveOn: "2026-06-01", PaymentOn: "2026-06-10",
		AmountValue: exact.New(1000), AmountScale: 2, SourceEvidence: json.RawMessage(`{"notice":"238"}`),
	}, http.StatusCreated)
	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		"/api/v1/investments/gains/statement?from=2026-06-01&to=2026-06-30", nil, http.StatusOK)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Contains(t, body, "capital_returns", "excluded excess must be visible in the period report itself")
	var statement investmentGainsStatementResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &statement))
	require.Len(t, statement.CapitalReturns, 1)
	require.Equal(t, "2026-06-01", statement.CapitalReturns[0].EffectiveOn)
	require.Equal(t, "2026-06-10", statement.CapitalReturns[0].PaymentOn)
	require.Equal(t, 1, statement.Rows[0].CapitalReturn.ReceiptCount)
	requireScaledAPIAmount(t, 1000, 2, &statement.Rows[0].CapitalReturn.Receipt.Value, &statement.Rows[0].CapitalReturn.Receipt.Scale)
	requireScaledAPIAmount(t, 700, 2, &statement.Rows[0].CapitalReturn.Reduction.Value, &statement.Rows[0].CapitalReturn.Reduction.Scale)
	requireScaledAPIAmount(t, 300, 2, &statement.Rows[0].CapitalReturn.Excess.Value, &statement.Rows[0].CapitalReturn.Excess.Scale)
	require.NotNil(t, statement.Rows[0].TotalValue)
	require.NotNil(t, statement.Rows[0].TotalScale)
	assertMoneyValue(t, 700, 2, *statement.Rows[0].TotalValue, *statement.Rows[0].TotalScale)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		"/api/v1/investments/gains/statement?from=2026-06-01&to=2026-06-30&reporting_currency_id="+strconv.FormatInt(f.commodityID, 10)+"&fx_method=historic_basis_valuation_close", nil, http.StatusOK)
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &statement))
	require.NotNil(t, statement.Reporting)
	requireScaledAPIAmount(t, 300, 2, &statement.Reporting.CapitalReturnExcess.Amount.Value, &statement.Reporting.CapitalReturnExcess.Amount.Scale)
	require.Equal(t, "2026-06-10", statement.Reporting.CapitalReturnExcess.Components[0].RequestedDate)
	require.Equal(t, "2026-06-01", statement.Reporting.CapitalReturnReduction.Components[0].RequestedDate)
}

func TestCapitalReturnStatementSchemaUpgradePreservesLegacyBytesAndNewFrozenDisclosure(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "ROCDOC")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 700)
	buy.TransactionDate = "2026-05-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/return-of-capital", capitalReturnRequest{
		HoldingAccountID: holding.ID, CommodityID: instrument.CommodityID, CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID,
		EffectiveOn: "2026-06-01", PaymentOn: "2026-06-10", AmountValue: exact.New(1000), AmountScale: 2, SourceEvidence: json.RawMessage(`{"notice":"238"}`),
	}, http.StatusCreated)
	var receipt capitalReturnResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &receipt))
	ctx := context.Background()
	service := app.NewInvestmentService(db.NewInvestmentRepository(database), nil, nil, nil)
	// Manufacture the schema-1/2 wire formats using the real computation,
	// then persist on migrations 7/8 as the old servers did.
	statement, err := service.GainsStatement(ctx, "2026-06-01", "2026-06-30")
	require.NoError(t, err)
	statement.Profile.From, statement.Profile.To = statement.From, statement.To
	currencies, err := app.NewCurrencyService(db.NewCommodityRepository(database), nil).ListCurrencies(ctx)
	require.NoError(t, err)
	var output map[string]json.RawMessage
	encoded, err := json.Marshal(gainsStatementResponse(statement, currencies))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(encoded, &output))
	var rows []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(output["rows"], &rows))
	for _, row := range rows {
		delete(row, "unclassified_dividends")
	}
	output["rows"], err = json.Marshal(rows)
	require.NoError(t, err)
	schema2Output, err := json.Marshal(output)
	require.NoError(t, err)
	delete(output, "capital_returns")
	for _, row := range rows {
		delete(row, "capital_return")
	}
	output["rows"], err = json.Marshal(rows)
	require.NoError(t, err)
	encoded, err = json.Marshal(output)
	require.NoError(t, err)
	profile := toGainsProfileResponse(statement.Profile)
	inputs, err := json.Marshal(map[string]any{"report_type": "statement", "from": statement.From, "to": statement.To, "profile": profile, "snapshot_sha256": strings.Repeat("a", 64)})
	require.NoError(t, err)
	var ownerID int64
	require.NoError(t, database.QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&ownerID))
	captured, err := time.Parse(time.RFC3339Nano, profile.KnowledgeCutoff)
	require.NoError(t, err)
	legacy := db.GainsRunRecord{BookID: 1, SchemaVersion: 1, ReportType: "statement", CapturedAt: profile.KnowledgeCutoff, CreatedAt: captured.Format(time.RFC3339),
		CreatedByUserID: ownerID, SnapshotSHA256: strings.Repeat("a", 64), InputsJSON: inputs, OutputJSON: encoded, InputsSHA256: db.GainsRunDigest(inputs), OutputSHA256: db.GainsRunDigest(encoded)}
	provider, err := goose.NewProvider(goose.DialectSQLite3, database, migrations.FS)
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 7)
	require.NoError(t, err)
	legacy, err = db.NewInvestmentRepository(database).CreateGainsRun(ctx, legacy, db.AuditEventParams{BookID: 1, ActorUserID: ownerID, OriginType: "browser_api", OccurredAt: legacy.CreatedAt})
	require.NoError(t, err)
	require.True(t, db.ValidGainsRunDocument(legacy))
	_, err = provider.UpTo(ctx, 8)
	require.NoError(t, err)
	legacy2 := legacy
	legacy2.SchemaVersion = 2
	legacy2.OutputJSON = schema2Output
	legacy2.OutputSHA256 = db.GainsRunDigest(schema2Output)
	legacy2, err = db.NewInvestmentRepository(database).CreateGainsRun(ctx, legacy2, db.AuditEventParams{BookID: 1, ActorUserID: ownerID, OriginType: "browser_api", OccurredAt: legacy2.CreatedAt})
	require.NoError(t, err)
	require.True(t, db.ValidGainsRunDocument(legacy2))
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	for _, run := range []db.GainsRunRecord{legacy, legacy2} {
		got, err := service.GainsRun(ctx, run.ID)
		require.NoError(t, err)
		require.Equal(t, run, got, "forward migration retains original bytes and hashes")
		res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains/runs/"+strconv.FormatInt(run.ID, 10)+"?download=true", nil, http.StatusOK)
		var downloaded db.GainsRunRecord
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &downloaded))
		downloaded.BookID = run.BookID
		require.Equal(t, run, downloaded, "retrieval never backfills an old schema")
	}
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/gains/runs", app.GainsRunRequest{ReportType: "statement", From: "2026-06-01", To: "2026-06-30"}, http.StatusCreated)
	var saved db.GainsRunRecord
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &saved))
	saved.BookID = 1
	require.Equal(t, 3, saved.SchemaVersion)
	require.Contains(t, string(saved.OutputJSON), `"unclassified_dividends":{"payment_count":0`, "schema 3 freezes the dividend disclosure")
	require.NotContains(t, string(legacy.OutputJSON), "unclassified_dividends")
	var savedStatement investmentGainsStatementResponse
	require.NoError(t, json.Unmarshal(saved.OutputJSON, &savedStatement))
	require.Len(t, savedStatement.CapitalReturns, 1)
	requireScaledAPIAmount(t, 300, 2, &savedStatement.Rows[0].CapitalReturn.Excess.Value, &savedStatement.Rows[0].CapitalReturn.Excess.Scale)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/transactions/"+strconv.FormatInt(receipt.Transaction.ID, 10)+"/reverse-return-of-capital", map[string]any{"reason": "withdrawn"}, http.StatusCreated)
	for _, run := range []db.GainsRunRecord{legacy, legacy2, saved} {
		got, err := service.GainsRun(ctx, run.ID)
		require.NoError(t, err)
		require.Equal(t, run, got)
	}
	live, err := service.GainsStatement(ctx, "2026-06-01", "2026-06-30")
	require.NoError(t, err)
	require.Empty(t, live.CapitalReturns)
	_, err = provider.DownTo(ctx, 7)
	require.Error(t, err, "cannot downgrade away schema-2 and schema-3 contracts")
	// Goose rolls back one migration at a time; later migrations can already
	// have been removed before schema 8 refuses. Restore the head before using
	// the current server's export contract.
	require.NoError(t, db.Migrate(ctx, database))
	// The portable export preserves all three saved schemas, even outside its ledger scope.
	bundle := downloadBundle(t, handler, f.sessionCookie, "?from=2026-07-01&to=2026-07-31")
	table := bundle.table(t, "investment-gains-runs.csv")
	require.Len(t, table.rows, 3)
	for i, run := range []db.GainsRunRecord{legacy, legacy2, saved} {
		require.Equal(t, string(run.OutputJSON), table.column(table.rows[i], "output_json"))
		require.Equal(t, strconv.Itoa(run.SchemaVersion), table.column(table.rows[i], "schema_version"))
	}
}
