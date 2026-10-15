package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"rekenraam/backend/internal/app"
)

// readDividendCoverage serves #244's composed income attribution read. A
// limit of 0 returns the summary only, as an instrument view needs.
func readDividendCoverage(logger *slog.Logger, authService *app.AuthService, investmentService *app.InvestmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedOwner(w, r, logger, authService); !ok {
			return
		}
		query := r.URL.Query()
		input := app.DividendCoverageInput{Cursor: query.Get("cursor")}
		if raw := query.Get("commodity_id"); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || value <= 0 {
				writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid instrument filter")
				return
			}
			input.CommodityID = value
		}
		if raw := query.Get("limit"); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 || value > 100 {
				writeAPIError(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid coverage limit")
				return
			}
			input.Limit, input.SummaryOnly = value, value == 0
		}
		result, err := investmentService.DividendCoverage(r.Context(), input)
		if err != nil {
			writeInvestmentServiceError(w, r, logger, "read income coverage", err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}
