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

func formatGainsRun(projection app.GainsRunProjection) (json.RawMessage, error) {
	if projection.Statement != nil {
		return json.Marshal(gainsStatementResponse(*projection.Statement, projection.Currencies))
	}
	response, err := gainsProjectionResponse(projection.Profile, projection.Realized, projection.Unrealized)
	if err != nil {
		return nil, err
	}
	response.CapitalReturns = toCapitalReturnReads(projection.CapitalReturns)
	response.Currencies = toCurrencyResponses(projection.Currencies)
	return json.Marshal(response)
}

func createInvestmentGainsRun(logger *slog.Logger, auth *app.AuthService, investments *app.InvestmentService, options HandlerOptions) http.HandlerFunc {
	return requireAuthenticatedMutation(logger, auth, options, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		owner, ok := authenticatedMutationOwner(w, r)
		if !ok {
			return
		}
		var request app.GainsRunRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeDecodeError(w, err)
			return
		}
		run, err := investments.CreateGainsRun(r.Context(), app.GainsRunInput{Request: request, OwnerUserID: owner.ID,
			AuthSessionID: authenticatedSessionID(r), RequestID: RequestIDFromContext(r.Context())}, formatGainsRun)
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "create gains run", err)
			return
		}
		w.Header().Set("Location", "/api/v1/investments/gains/runs/"+strconv.FormatInt(run.ID, 10))
		writeJSON(w, http.StatusCreated, run)
	}))
}

func getInvestmentGainsRun(logger *slog.Logger, auth *app.AuthService, investments *app.InvestmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedOwner(w, r, logger, auth); !ok {
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid gains run ID")
			return
		}
		run, err := investments.GainsRun(r.Context(), id)
		if errors.Is(err, app.ErrGainsRunNotFound) {
			writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "gains run not found")
			return
		}
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "get gains run", err)
			return
		}
		if r.URL.Query().Get("download") == "true" {
			w.Header().Set("Content-Disposition", `attachment; filename="rekenraam-gains-run-`+strconv.FormatInt(run.ID, 10)+`.json"`)
		}
		writeJSON(w, http.StatusOK, run)
	}
}

func listInvestmentGainsRuns(logger *slog.Logger, auth *app.AuthService, investments *app.InvestmentService) http.HandlerFunc {
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
				writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid gains run cursor")
				return
			}
		}
		runs, next, err := investments.ListGainsRuns(r.Context(), before)
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "list gains runs", err)
			return
		}
		var cursor *string
		if next > 0 {
			value := strconv.FormatInt(next, 10)
			cursor = &value
		}
		writeJSON(w, http.StatusOK, struct {
			Runs       []db.GainsRunRecord `json:"runs"`
			NextCursor *string             `json:"next_cursor"`
		}{runs, cursor})
	}
}
