package api

import (
	"net/url"
	"strconv"

	"rekenraam/backend/internal/app"
)

type gainsFXProjectionResponse struct {
	Profile         gainsProfileResponse `json:"profile"`
	Realized        []app.GainsFXFigure  `json:"realized"`
	Unrealized      []app.GainsFXFigure  `json:"unrealized"`
	RealizedTotal   app.GainsFXFigure    `json:"realized_total"`
	UnrealizedTotal app.GainsFXFigure    `json:"unrealized_total"`
}
type gainsFXStatementResponse struct {
	CapitalReturnReceipt   app.GainsFXFigure    `json:"capital_return_receipt"`
	CapitalReturnReduction app.GainsFXFigure    `json:"capital_return_reduction"`
	CapitalReturnExcess    app.GainsFXFigure    `json:"capital_return_excess"`
	Profile                gainsProfileResponse `json:"profile"`
	Opening                app.GainsFXFigure    `json:"opening"`
	Realized               app.GainsFXFigure    `json:"realized"`
	Income                 app.GainsFXFigure    `json:"income"`
	Closing                app.GainsFXFigure    `json:"closing"`
	UnrealizedChange       app.GainsFXFigure    `json:"unrealized_change"`
	Total                  app.GainsFXFigure    `json:"total"`
}

func gainsFXInput(query url.Values) (*app.GainsFXInput, error) {
	active := false
	for _, key := range []string{"reporting_currency_id", "fx_method", "fx_max_age_days"} {
		if values, ok := query[key]; ok {
			active = true
			if len(values) != 1 || values[0] == "" {
				return nil, app.ValidationError{Message: "FX parameters must occur once and cannot be blank"}
			}
		}
	}
	if !active {
		return nil, nil
	}
	id, err := strconv.ParseInt(query.Get("reporting_currency_id"), 10, 64)
	if err != nil || id <= 0 {
		return nil, app.ValidationError{Message: "reporting currency must be a positive ID"}
	}
	age := 7
	if raw := query.Get("fx_max_age_days"); raw != "" {
		age, err = strconv.Atoi(raw)
		if err != nil || age < 1 || age > 365 {
			return nil, app.ValidationError{Message: "FX maximum age must be between 1 and 365 days"}
		}
	}
	return &app.GainsFXInput{CommodityID: id, Method: query.Get("fx_method"), MaxAgeDays: age}, nil
}
