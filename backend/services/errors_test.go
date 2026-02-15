package services

import (
	"errors"
	"reflect"
	"testing"
)

func TestAppErrorNilReceiver(t *testing.T) {
	var appErr *AppError

	if got := appErr.Error(); got != "" {
		t.Fatalf("expected empty string for nil receiver, got %q", got)
	}
	if appErr.Unwrap() != nil {
		t.Fatalf("expected nil unwrap for nil receiver")
	}
}

func TestAppErrorErrorWithWrappedError(t *testing.T) {
	root := errors.New("db down")
	appErr := &AppError{HTTPCode: 500, Message: "query failed", Err: root}

	if got := appErr.Error(); got != "query failed: db down" {
		t.Fatalf("unexpected error text: %q", got)
	}
	if !errors.Is(appErr, root) {
		t.Fatalf("expected wrapped error to be discoverable via errors.Is")
	}
}

func TestNewAppErrorWithData(t *testing.T) {
	payload := map[string]string{"field": "name"}
	err := newAppErrorWithData(400, "bad request", payload, nil)

	if err.HTTPCode != 400 {
		t.Fatalf("expected HTTPCode 400, got %d", err.HTTPCode)
	}
	if err.Message != "bad request" {
		t.Fatalf("unexpected message: %q", err.Message)
	}
	if !reflect.DeepEqual(err.Data, payload) {
		t.Fatalf("expected data payload to be preserved")
	}
}
