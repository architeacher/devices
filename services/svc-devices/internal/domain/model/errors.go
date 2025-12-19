package model

import "errors"

var (
	ErrDeviceNotFound          = errors.New("device not found")
	ErrCannotUpdateInUseDevice = errors.New("cannot update name or brand of in-use device")
	ErrCannotDeleteInUseDevice = errors.New("cannot delete in-use device")
	ErrInvalidDeviceID         = errors.New("invalid device ID")
	ErrInvalidState            = errors.New("invalid device state")
	ErrDuplicateDevice         = errors.New("device already exists")
	ErrDatabaseConnection      = errors.New("database connection error")
	ErrDatabaseQuery           = errors.New("database query error")
)

type ValidationError struct {
	Field   string
	Message string
	Code    string
}

type ValidationErrors struct {
	Errors []ValidationError
}

func (v *ValidationErrors) Error() string {
	if len(v.Errors) == 0 {
		return "validation failed"
	}

	return v.Errors[0].Message
}

func (v *ValidationErrors) Add(field, message, code string) {
	v.Errors = append(v.Errors, ValidationError{
		Field:   field,
		Message: message,
		Code:    code,
	})
}

func (v *ValidationErrors) HasErrors() bool {
	return len(v.Errors) > 0
}

func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{
		Errors: make([]ValidationError, 0),
	}
}
