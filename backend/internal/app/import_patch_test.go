package app

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
)

const twoRowPatchQIF = "!Type:Bank\nD2026-08-28\nT-12.34\nPBooks\n^\nD2026-08-29\nT-17.00\nPTravel\n^\n"

func stageTwoRowPatchBatch(t *testing.T, f *plainImportTestFixture) (batchID int64, rowIDs []int64) {
	t.Helper()
	result, err := f.importService.StartImport(context.Background(), StartImportInput{
		OwnerUserID: f.ownerUserID,
		Input:       RawInput{Filename: "test.qif", Bytes: []byte(twoRowPatchQIF)},
	})
	require.NoError(t, err)
	require.Len(t, result.Rows, 2)
	return result.Batch.ID, []int64{result.Rows[0].ID, result.Rows[1].ID}
}

func patchTestResolutionJSON(f *plainImportTestFixture) string {
	return `{"account_id":` + strconv.FormatInt(f.checkingAccountID, 10) +
		`,"commodity_id":` + strconv.FormatInt(f.eurCommodityID, 10) +
		`,"category_id":` + strconv.FormatInt(f.categoryAccountID, 10) + `}`
}

func stagedRowReviewState(t *testing.T, f *plainImportTestFixture, rowID int64) (dedupeStatus, resolutionJSON string) {
	t.Helper()
	require.NoError(t, f.database.QueryRowContext(context.Background(),
		`SELECT dedupe_status, resolution_json FROM import_staged_rows WHERE id = ?`, rowID,
	).Scan(&dedupeStatus, &resolutionJSON))
	return dedupeStatus, resolutionJSON
}

// #243: a direct-service patch that omits dedupe_status or resolution gets
// the same documented defaults as the HTTP handler instead of a raw CHECK
// error from the repository.
func TestPatchImportBatch_OmittedFieldsTakeDocumentedDefaults(t *testing.T) {
	t.Parallel()
	f := newPlainImportTestFixture(t)
	ctx := context.Background()
	batchID, rowIDs := stageTwoRowPatchBatch(t, f)
	resolution := patchTestResolutionJSON(f)

	// Resolution-only patch: dedupe_status defaults to "new".
	require.NoError(t, f.importService.PatchImportBatch(ctx, PatchImportBatchInput{
		OwnerUserID:    f.ownerUserID,
		BatchID:        batchID,
		RowResolutions: []RowResolutionPatch{{RowID: rowIDs[0], ResolutionJSON: resolution}},
	}))
	status, stored := stagedRowReviewState(t, f, rowIDs[0])
	assert.Equal(t, "new", status)
	assert.JSONEq(t, resolution, stored)

	// Status-only patch: resolution defaults to "{}" (a patch replaces the
	// row's whole review state).
	require.NoError(t, f.importService.PatchImportBatch(ctx, PatchImportBatchInput{
		OwnerUserID:    f.ownerUserID,
		BatchID:        batchID,
		RowResolutions: []RowResolutionPatch{{RowID: rowIDs[0], DedupeStatus: "excluded"}},
	}))
	status, stored = stagedRowReviewState(t, f, rowIDs[0])
	assert.Equal(t, "excluded", status)
	assert.JSONEq(t, `{}`, stored)
}

// #243: every patch is validated, and the batch applied atomically, so a
// later invalid or unknown row cannot leave earlier choices half-applied.
func TestPatchImportBatch_RejectedPatchAppliesNothing(t *testing.T) {
	t.Parallel()
	resolution := `{"account_id":1}`

	cases := []struct {
		name   string
		second func(otherBatchRowID int64) RowResolutionPatch
	}{
		{"invalid dedupe status", func(int64) RowResolutionPatch {
			return RowResolutionPatch{RowID: 0, DedupeStatus: "bogus"}
		}},
		{"invalid resolution JSON", func(int64) RowResolutionPatch {
			return RowResolutionPatch{RowID: 1, ResolutionJSON: "{"}
		}},
		{"row of another batch", func(otherBatchRowID int64) RowResolutionPatch {
			return RowResolutionPatch{RowID: otherBatchRowID, DedupeStatus: "excluded"}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newPlainImportTestFixture(t)
			ctx := context.Background()
			batchID, rowIDs := stageTwoRowPatchBatch(t, f)
			_, otherRowIDs := stageTwoRowPatchBatch(t, f)
			beforeStatus, beforeResolution := stagedRowReviewState(t, f, rowIDs[0])

			err := f.importService.PatchImportBatch(ctx, PatchImportBatchInput{
				OwnerUserID: f.ownerUserID,
				BatchID:     batchID,
				RowResolutions: []RowResolutionPatch{
					{RowID: rowIDs[0], DedupeStatus: "excluded", ResolutionJSON: resolution},
					tc.second(otherRowIDs[0]),
				},
			})
			_, isValidation := err.(ValidationError)
			require.True(t, isValidation, "want ValidationError, got %v", err)

			status, stored := stagedRowReviewState(t, f, rowIDs[0])
			assert.Equal(t, beforeStatus, status)
			assert.JSONEq(t, beforeResolution, stored)
			otherStatus, _ := stagedRowReviewState(t, f, otherRowIDs[0])
			assert.Equal(t, "new", otherStatus)
		})
	}
}

// #243: the repository re-checks the batch state inside its transaction, so
// a commit that lands after the service's admission check rejects the whole
// patch instead of rewriting rows the commit already read.
func TestPatchImportStagedRowResolutions_ClosedBatchRejectsWholePatch(t *testing.T) {
	t.Parallel()
	f := newPlainImportTestFixture(t)
	ctx := context.Background()
	batchID, rowIDs := stageTwoRowPatchBatch(t, f)
	_, err := f.database.ExecContext(ctx, `UPDATE import_batches SET status = 'committed' WHERE id = ?`, batchID)
	require.NoError(t, err)

	err = f.importRepo.PatchImportStagedRowResolutions(ctx, batchID, []db.UpdateImportStagedRowResolutionParams{
		{RowID: rowIDs[0], BatchID: batchID, DedupeStatus: "excluded", ResolutionJSON: "{}"},
	})
	require.ErrorIs(t, err, db.ErrImportBatchNotPreviewing)
	status, _ := stagedRowReviewState(t, f, rowIDs[0])
	assert.Equal(t, "new", status)
}
