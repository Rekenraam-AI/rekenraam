package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// recordedOperation is what a command reported: the kind it records, its
// primary journal, its event date and, for a reversal, the journal it undoes.
type recordedOperation struct {
	kind          string
	transactionID int64
	eventDate     string
	reverses      int64
}

// capabilityScenarios record every operation kind across a few books. Each
// family gets its own book so one family's whole-holding operation cannot
// refuse another's.
var capabilityScenarios = []struct {
	name   string
	record func(t *testing.T, f *investmentsTestFixture) []recordedOperation
}{
	{"trades and income", func(t *testing.T, f *investmentsTestFixture) []recordedOperation {
		ctx := context.Background()
		buy := buyOn(t, f, "2026-01-02", 10, 10000)
		sale := acknowledgedSell(t, f, sellInput(f, "2026-02-01", 1))
		writeOff := writeOffOn(t, f, "2026-02-02", 1, "fifo")
		dividendInput := cashDividendInput(f, 0, 10000, 0)
		dividendInput.CommodityID = &f.stockCommodityID
		dividend, err := f.investmentService.Dividend(ctx, dividendInput)
		require.NoError(t, err)
		taxAccount := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
		taxInput := dividendInput
		taxInput.WithholdingOnly, taxInput.AmountValue = true, 1500
		taxInput.IncomeAccountID, taxInput.WithholdingAccountID = nil, &taxAccount
		tax, err := f.investmentService.Dividend(ctx, taxInput)
		require.NoError(t, err)
		reinvestment := reinvestOn(t, f, "2026-03-02", 1, 500)
		capitalReturn, err := f.investmentService.CapitalReturn(ctx, capitalReturnInput(f, "2026-04-01", 400))
		require.NoError(t, err)
		reversal, err := f.investmentService.ReverseDividend(ctx, ReverseInvestmentDividendInput{
			OwnerUserID: f.ownerUserID, TransactionID: dividend.ID, Reason: "entered twice"})
		require.NoError(t, err)
		return []recordedOperation{
			{"buy", buy.Transaction.ID, "2026-01-02", 0},
			{"sell", sale.Transaction.ID, "2026-02-01", 0},
			{"write_off", writeOff.Transaction.ID, "2026-02-02", 0},
			{"dividend", dividend.ID, "2026-03-01", 0},
			{"dividend", tax.ID, "2026-03-01", 0},
			{"reinvested_dividend", reinvestment.Transaction.ID, "2026-03-02", 0},
			{"return_of_capital", capitalReturn.Transaction.ID, "2026-04-01", 0},
			{"reversal", reversal.ID, "2026-03-01", dividend.ID},
		}
	}},
	{"corporate actions", func(t *testing.T, f *investmentsTestFixture) []recordedOperation {
		ctx := context.Background()
		buy := buyOn(t, f, "2026-05-01", 3, 3000)
		split, err := f.investmentService.Split(ctx, splitInput(f, "2026-06-01", 3, 2))
		require.NoError(t, err)
		cashInLieu, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, split.Transaction.ID))
		require.NoError(t, err)
		spinOff, err := f.investmentService.SpinOff(ctx,
			spinOffInput(f, seedTestSecurityCommodity(t, f, "CHILD"), "2026-07-01", 1, 1, "25", 2))
		require.NoError(t, err)
		exchange, err := f.investmentService.ShareExchange(ctx,
			shareExchangeInput(f, seedTestSecurityCommodity(t, f, "NEWCO"), "2026-08-01", 1, 1))
		require.NoError(t, err)
		return []recordedOperation{
			{"buy", buy.Transaction.ID, "2026-05-01", 0},
			{"split", split.Transaction.ID, "2026-06-01", 0},
			{"cash_in_lieu", cashInLieu.Transaction.ID, "2026-06-01", 0},
			{"spin_off", spinOff.Transaction.ID, "2026-07-01", 0},
			{"share_exchange", exchange.Transaction.ID, "2026-08-01", 0},
		}
	}},
	{"cash and stock merger", func(t *testing.T, f *investmentsTestFixture) []recordedOperation {
		buy := buyOn(t, f, "2026-02-01", 100, 100000)
		merger := defaultCashStockMerger(t, f, seedTestSecurityCommodity(t, f, "NEWCO"))
		return []recordedOperation{
			{"buy", buy.Transaction.ID, "2026-02-01", 0},
			{"cash_and_stock_merger", merger.Transaction.ID, "2026-06-01", 0},
		}
	}},
	{"transfers", func(t *testing.T, f *investmentsTestFixture) []recordedOperation {
		ctx := context.Background()
		seedExternalTransferEquity(t, f.database)
		destination := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
		buy := buyOn(t, f, "2026-01-02", 10, 10000)
		internal, err := f.investmentService.InternalTransfer(ctx, internalTransferFromLot(f, destination, *buy.LotID, exact.New(3), 0))
		require.NoError(t, err)
		outbound, err := f.investmentService.ExternalTransferOut(ctx, transferOutOfLot(f, "2026-06-02", *buy.LotID, 2))
		require.NoError(t, err)
		inbound, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-06-03", "2020-03-01"))
		require.NoError(t, err)
		resolution := resolveAcknowledged(t, f, resolveInput(f, inbound.Transaction.ID, 8000))
		return []recordedOperation{
			{"buy", buy.Transaction.ID, "2026-01-02", 0},
			{"internal_transfer", internal.Transaction.ID, "2026-06-01", 0},
			{"external_transfer_out", outbound.Transaction.ID, "2026-06-02", 0},
			{"external_transfer_in", inbound.Transaction.ID, "2026-06-03", 0},
			{"basis_resolution", resolution.ID, "2026-06-03", 0},
		}
	}},
	{"shorts", func(t *testing.T, f *investmentsTestFixture) []recordedOperation {
		opening := shortSaleOn(t, f, "2026-03-02", 10, 10000)
		cover, err := f.investmentService.ShortCover(context.Background(), coverInput(f, "2026-05-01", 4, 3000, "fifo"))
		require.NoError(t, err)
		expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
		payment, err := f.investmentService.PaymentInLieu(context.Background(), paymentInLieuInput(f, expense, 5000))
		require.NoError(t, err)
		return []recordedOperation{
			{"short_sale", opening.Transaction.ID, "2026-03-02", 0},
			{"payment_in_lieu", payment.ID, "2026-04-03", 0},
			{"short_cover", cover.Transaction.ID, "2026-05-01", 0},
		}
	}},
}

// exportedBundle is every CSV of the export bundle, as rows keyed by header.
func exportedBundle(t *testing.T, f *investmentsTestFixture) map[string][]map[string]string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, NewExportService(db.NewExportRepository(f.database)).WriteBundle(context.Background(), &out, ExportFilter{}))
	archive, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	require.NoError(t, err)
	files := make(map[string][]map[string]string)
	for _, file := range archive.File {
		if !strings.HasSuffix(file.Name, ".csv") {
			continue
		}
		reader, err := file.Open()
		require.NoError(t, err)
		rows, err := csv.NewReader(reader).ReadAll()
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		rows[0][0] = strings.TrimPrefix(rows[0][0], utf8ByteOrderMark)
		records := make([]map[string]string, 0, len(rows))
		for _, row := range rows[1:] {
			record := make(map[string]string, len(row))
			for index, column := range rows[0] {
				record[column] = row[index]
			}
			records = append(records, record)
		}
		files[file.Name] = records
	}
	return files
}

func exportedOperationIDs(files map[string][]map[string]string, name string) map[string]bool {
	ids := make(map[string]bool)
	for _, row := range files[name] {
		ids[row["operation_id"]] = true
	}
	return ids
}

// The export and self-check columns of the capability matrix, and its
// lot-free replay cells: one operation of every accepted kind is recorded,
// the bundle must carry exactly those operations with their journals, dates
// and correction links plus each row's family files, and every self-check
// and the conservation oracle must pass.
func TestEveryOperationKindExportsItsRowsAndPassesSelfCheck(t *testing.T) {
	t.Parallel()
	rows := make(map[string]operationKindCapabilities, len(operationKindCapabilityMatrix))
	for _, row := range operationKindCapabilityMatrix {
		rows[row.kind] = row
	}
	recordedKinds := make(chan []string, len(capabilityScenarios))
	t.Run("scenarios", func(t *testing.T) {
		for _, scenario := range capabilityScenarios {
			t.Run(scenario.name, func(t *testing.T) {
				t.Parallel()
				f := newInvestmentsTestFixture(t)
				recorded := scenario.record(t, f)
				files := exportedBundle(t, f)

				operations := files["investment-operations.csv"]
				require.Len(t, operations, len(recorded), "the bundle carries exactly the recorded operations")
				byTransaction := make(map[string]map[string]string, len(operations))
				for _, operation := range operations {
					byTransaction[operation["transaction_id"]] = operation
				}
				kinds := make([]string, 0, len(recorded))
				for _, expected := range recorded {
					kinds = append(kinds, expected.kind)
					exported, found := byTransaction[strconv.FormatInt(expected.transactionID, 10)]
					require.True(t, found, "%s: no exported operation for transaction %d", expected.kind, expected.transactionID)
					require.Equal(t, expected.kind, exported["operation_kind"])
					require.Equal(t, expected.eventDate, exported["event_date"], expected.kind)
					if expected.reverses == 0 {
						require.Empty(t, exported["correction_mode"], expected.kind)
					} else {
						original := byTransaction[strconv.FormatInt(expected.reverses, 10)]
						require.Equal(t, "reverse", exported["correction_mode"])
						require.Equal(t, original["operation_id"], exported["correction_of_operation_id"])
					}
					id := exported["operation_id"]
					require.True(t, exportedOperationIDs(files, "investment-operation-journal-links.csv")[id], "%s journal link", expected.kind)
					require.True(t, exportedOperationIDs(files, "investment-operation-dates.csv")[id], "%s dates", expected.kind)
					row, found := rows[expected.kind]
					require.True(t, found, "%s has no capability matrix row", expected.kind)
					for _, name := range row.exportedIn {
						require.True(t, exportedOperationIDs(files, name)[id], "%s: %s does not name operation %s", expected.kind, name, id)
					}
					if row.replay.mode == capabilityLotFree {
						for _, name := range []string{lotFactsFile, "investment-lot-effects.csv", disposalDecisionsFile} {
							require.False(t, exportedOperationIDs(files, name)[id], "%s is lot-free but %s names it", expected.kind, name)
						}
					}
				}
				requireSelfCheckPasses(t, f)
				recordedKinds <- kinds
			})
		}
	})
	close(recordedKinds)
	var covered []string
	for kinds := range recordedKinds {
		covered = append(covered, kinds...)
	}
	for _, kind := range db.InvestmentOperationKinds {
		require.Contains(t, covered, kind, "no scenario records %s", kind)
	}
}

// The reversal row's correction cells: a reversal is terminal. Its own
// journal offers no correction, and the family commands and the generic
// lifecycle refuse it by name.
func TestReversalOperationIsNotItselfCorrectable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-01-02", 10, 10000)
	sale := acknowledgedSell(t, f, sellInput(f, "2026-02-01", 4))
	reversal, err := acknowledgedReverseSale(ctx, f.investmentService, ReverseInvestmentSaleInput{
		OwnerUserID: f.ownerUserID, TransactionID: sale.Transaction.ID, Reason: "never traded"})
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)

	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, reversal.ID)
	require.NoError(t, err)
	require.Nil(t, chain.EffectiveTransactionID, "a reversed chain has no effective operation")
	flags := reflect.ValueOf(chain)
	for index := range flags.NumField() {
		field := flags.Type().Field(index)
		if strings.HasPrefix(field.Name, "Can") && field.Type.Kind() == reflect.Bool {
			require.False(t, flags.Field(index).Bool(), "%s is offered for a reversal", field.Name)
		}
	}

	_, err = f.investmentService.ReverseSale(ctx, ReverseInvestmentSaleInput{OwnerUserID: f.ownerUserID,
		TransactionID: reversal.ID, Reason: "undo the reversal"})
	require.ErrorIs(t, err, ErrInvestmentSaleNotFound)
	_, err = f.investmentService.ReplaceSale(ctx, ReplaceInvestmentSaleInput{OwnerUserID: f.ownerUserID,
		TransactionID: reversal.ID, Reason: "undo the reversal", Replacement: sellInput(f, "2026-02-01", 4)})
	require.ErrorIs(t, err, ErrInvestmentSaleNotFound)
	_, err = f.transactionService.VoidTransaction(ctx, VoidTransactionInput{OwnerUserID: f.ownerUserID,
		TransactionID: reversal.ID, ChangeReason: "undo the reversal"})
	require.ErrorIs(t, err, ErrInvestmentWorkflowRequired)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "nothing written")
}
