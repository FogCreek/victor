package events

import "errors"

type ErrorEvent interface {
	IsFatal() bool
	ErrorObject() error
	error
}

type InvalidAuth struct{}

func (i *InvalidAuth) Error() string {
	return i.ErrorObject().Error()
}

func (i *InvalidAuth) ErrorObject() error {
	return errors.New("Invalid Auth")
}

func (i *InvalidAuth) IsFatal() bool {
	return true
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
