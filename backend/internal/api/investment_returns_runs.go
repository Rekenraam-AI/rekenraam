package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"rekenraam/backend/internal/app"
	"rekenraam/backend/internal/db"
)

// formatReturnsRun freezes the live read's own wire shape, so a reopened run
// renders with the same views as a live one.
func formatReturnsRun(result app.ReturnsResult) (json.RawMessage, error) {
	return json.Marshal(toInvestmentReturnsResponse(result))
}

func createInvestmentReturnsRun(logger *slog.Logger, auth *app.AuthService, investments *app.InvestmentService, options HandlerOptions) http.HandlerFunc {
	return requireAuthenticatedMutation(logger, auth, options, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		owner, ok := authenticatedMutationOwner(w, r)
		if !ok {
			return
		}
		var request app.ReturnsRunRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeDecodeError(w, err)
			return
		}
		run, err := investments.CreateReturnsRun(r.Context(), app.ReturnsRunInput{Request: request, OwnerUserID: owner.ID,
			AuthSessionID: authenticatedSessionID(r), RequestID: RequestIDFromContext(r.Context())}, formatReturnsRun)
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "create returns run", err)
			return
		}
		w.Header().Set("Location", "/api/v1/investments/returns/runs/"+strconv.FormatInt(run.ID, 10))
		writeJSON(w, http.StatusCreated, run)
	}))
}

func getInvestmentReturnsRun(logger *slog.Logger, auth *app.AuthService, investments *app.InvestmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedOwner(w, r, logger, auth); !ok {
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid returns run ID")
			return
		}
		run, err := investments.ReturnsRun(r.Context(), id)
		if errors.Is(err, app.ErrReturnsRunNotFound) {
			writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "returns run not found")
			return
		}
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "get returns run", err)
			return
		}
		if r.URL.Query().Get("download") == "true" {
			w.Header().Set("Content-Disposition", `attachment; filename="rekenraam-returns-run-`+strconv.FormatInt(run.ID, 10)+`.json"`)
		}
		writeJSON(w, http.StatusOK, run)
	}
}

func listInvestmentReturnsRuns(logger *slog.Logger, auth *app.AuthService, investments *app.InvestmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedOwner(w, r, logger, auth); !ok {
			return
		}
		var before int64
		if values, exists := r.URL.Query()["cursor"]; exists {
			var err error
			if len(values) == 1 {
				before, err = strconv.ParseInt(values[0], 10, 64)
			}
			if len(values) != 1 || err != nil || before <= 0 {
				writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid returns run cursor")
				return
			}
		}
		runs, next, err := investments.ListReturnsRuns(r.Context(), before)
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "list returns runs", err)
			return
		}
		var cursor *string
		if next > 0 {
			value := strconv.FormatInt(next, 10)
			cursor = &value
		}
		writeJSON(w, http.StatusOK, struct {
			Runs       []db.ReturnsRunRecord `json:"runs"`
			NextCursor *string               `json:"next_cursor"`
		}{runs, cursor})
	}
}
