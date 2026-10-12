package db

import (
	"errors"

	sqlite "github.com/mattn/go-sqlite3"
)

// IsSQLiteBusy includes extended BUSY and LOCKED results, including snapshot
// contention. Callers can return a retryable error without inspecting wording.
func IsSQLiteBusy(err error) bool {
	var native sqlite.Error
	return errors.As(err, &native) && (native.Code == sqlite.ErrBusy || native.Code == sqlite.ErrLocked)
}

func IsSQLiteUniqueConstraint(err error) bool {
	var native sqlite.Error
	return errors.As(err, &native) && (native.ExtendedCode == sqlite.ErrConstraintUnique || native.ExtendedCode == sqlite.ErrConstraintPrimaryKey)
}

func isSQLiteForeignKeyConstraint(err error) bool {
	var native sqlite.Error
	return errors.As(err, &native) && native.ExtendedCode == sqlite.ErrConstraintForeignKey
}
