package events

type ErrorEvent interface {
	IsFatal() bool
	ErrorObject() error
	error
}

// BaseError provides a bare set/get implementation of the ErrorEvent
// interface.
type BaseError struct {
	ErrorObj     error
	ErrorIsFatal bool
}

// Error returns the error event's underlying error's Error method.
func (b *BaseError) Error() string {
	return b.ErrorObj.Error()
}

// ErrorObject returns the original error that this error event wraps.
func (b *BaseError) ErrorObject() error {
	return b.ErrorObj
}

// IsFatal returns true if the error is unrecoverable by the chat adapter and
// false otherwise.
func (b *BaseError) IsFatal() bool {
	return b.ErrorIsFatal
}
