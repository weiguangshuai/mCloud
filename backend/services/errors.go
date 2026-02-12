package services

import "fmt"

type AppError struct {
	HTTPCode int
	Message  string
	Data     interface{}
	Err      error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newAppError(httpCode int, message string, err error) *AppError {
	return &AppError{HTTPCode: httpCode, Message: message, Err: err}
}

func newAppErrorWithData(httpCode int, message string, data interface{}, err error) *AppError {
	return &AppError{HTTPCode: httpCode, Message: message, Data: data, Err: err}
}
