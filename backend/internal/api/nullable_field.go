package api

import (
	"encoding/json"

	"rekenraam/backend/internal/app"
)

// Shared by nullable entry/update fields. omitzero preserves omission when a
// request is marshalled; explicit null remains present.
type nullableField[T any] struct{ app.NullablePatch[T] }

func (field *nullableField[T]) UnmarshalJSON(data []byte) error {
	field.Set = true
	return json.Unmarshal(data, &field.Value)
}

func (field nullableField[T]) MarshalJSON() ([]byte, error) { return json.Marshal(field.Value) }
func (field nullableField[T]) IsZero() bool                 { return !field.Set }
