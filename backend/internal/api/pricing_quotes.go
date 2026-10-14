package api

import (
	"log/slog"
	"net/http"
	"rekenraam/backend/internal/app"
)

type quoteRefreshAccepted struct {
	QueuedCount int `json:"queued_count"`
}

func queueQuoteRefresh(logger *slog.Logger, auth *app.AuthService, pricing *app.PricingService, options HandlerOptions) http.HandlerFunc {
	return requireAuthenticatedMutation(logger, auth, options, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		owner, ok := authenticatedMutationOwner(w, r)
		if !ok {
			return
		}
		var body struct{}
		if err := decodeJSONBody(r, &body); err != nil {
			writeDecodeError(w, err)
			return
		}
		count, err := pricing.QueueQuoteRefresh(r.Context(), owner.ID)
		if err != nil {
			writePricingServiceError(w, r, logger, "queue quote refresh", err)
			return
		}
		writeJSON(w, http.StatusAccepted, quoteRefreshAccepted{QueuedCount: count})
	}))
}
