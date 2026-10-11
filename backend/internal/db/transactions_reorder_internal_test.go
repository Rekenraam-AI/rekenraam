package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// placeInScope is the same-day rule behind #185's reorder: move the subject's
// items next to the anchor's only when the relation does not already hold,
// reusing the scope's sequence values.
func TestPlaceInScope(t *testing.T) {
	t.Parallel()
	item := func(transactionID, postingLineID, seq int64) reorderItem {
		return reorderItem{transactionID: transactionID, postingLineID: postingLineID, seq: seq}
	}
	day := []reorderItem{item(1, 11, 1), item(2, 21, 2), item(3, 31, 3)}

	assert.Empty(t, placeInScope(day, 1, 3, true), "already before")
	assert.Equal(t, []reorderItem{item(3, 31, 1), item(1, 11, 2), item(2, 21, 3)}, placeInScope(day, 3, 1, true),
		"moved before the anchor; everything between shifts by one")
	assert.Equal(t, []reorderItem{item(2, 21, 1), item(1, 11, 2)}, placeInScope(day, 1, 2, false), "after the anchor")
	assert.Empty(t, placeInScope(day, 4, 1, true), "a subject outside the scope")
	assert.Empty(t, placeInScope(day, 1, 4, true), "an anchor outside the scope")

	// A subject with two postings in the account moves as one block, in its own
	// order; gaps in the sequence values are kept.
	split := []reorderItem{item(1, 11, 2), item(2, 21, 5), item(2, 22, 7), item(3, 31, 9)}
	assert.Equal(t, []reorderItem{item(2, 21, 2), item(2, 22, 5), item(1, 11, 7)}, placeInScope(split, 2, 1, true))
}
