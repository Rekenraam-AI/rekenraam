package db

import (
	"errors"
	"fmt"
	"slices"
)

// ErrUnknownInvestmentOperationKind reports an operation kind no writer is
// allowed to record.
var ErrUnknownInvestmentOperationKind = errors.New("unknown investment operation kind")

// InvestmentOperationKinds are the operation kinds investment writers record.
// A new kind starts here, and the app package's capability matrix (#191) then
// fails until the kind has a row whose tests exercise or refuse its replay,
// replacement, reversal, export and self-check.
var InvestmentOperationKinds = []string{
	"buy",
	"sell",
	"write_off",
	"cash_in_lieu",
	"dividend",
	"payment_in_lieu",
	"reinvested_dividend",
	"return_of_capital",
	"split",
	"short_sale",
	"short_cover",
	"share_exchange",
	"spin_off",
	"cash_and_stock_merger",
	"internal_transfer",
	"external_transfer_in",
	"external_transfer_out",
	"basis_resolution",
	"reversal",
}

func requireKnownInvestmentOperationKind(kind string) error {
	if !slices.Contains(InvestmentOperationKinds, kind) {
		return fmt.Errorf("%w: %q", ErrUnknownInvestmentOperationKind, kind)
	}
	return nil
}
