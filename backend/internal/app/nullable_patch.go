package app

// NullablePatch distinguishes omission (preserve) from explicit null (clear).
// JSON decoding belongs to the API; services retain the same distinction.
type NullablePatch[T any] struct {
	Set   bool
	Value *T
}

func NullableValue[T any](value T) NullablePatch[T] {
	return NullablePatch[T]{Set: true, Value: &value}
}

func (field NullablePatch[T]) OrZero() T {
	if field.Value != nil {
		return *field.Value
	}
	var zero T
	return zero
}
