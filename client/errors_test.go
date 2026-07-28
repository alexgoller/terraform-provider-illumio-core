// Copyright 2021 Illumio, Inc. All Rights Reserved.

package client

import (
	"errors"
	"fmt"
	"testing"
)

func TestNotFoundErrorIsErrNotFound(t *testing.T) {
	err := error(&NotFoundError{Resource: "/orgs/1/sec_policy/draft/ip_lists/7"})

	if !errors.Is(err, ErrNotFound) {
		t.Error("errors.Is(err, ErrNotFound) = false, want true")
	}

	// Must survive wrapping, since callers wrap client errors with context.
	wrapped := fmt.Errorf("reading ip list: %w", err)
	if !errors.Is(wrapped, ErrNotFound) {
		t.Error("errors.Is on a wrapped error = false, want true")
	}
}

func TestOtherErrorsAreNotErrNotFound(t *testing.T) {
	for _, err := range []error{
		errors.New("unauthorized: please check your credentials"),
		errors.New("forbidden: you do not have permission OR org_id is invalid"),
		fmt.Errorf("failed: status code: %d", 500),
	} {
		if errors.Is(err, ErrNotFound) {
			t.Errorf("errors.Is(%v, ErrNotFound) = true, want false", err)
		}
	}
}

// The message must not change: it is user-facing and appears in existing docs
// and issue reports.
func TestNotFoundErrorMessageUnchanged(t *testing.T) {
	err := &NotFoundError{Resource: "/orgs/1/sec_policy/draft/ip_lists/7"}

	want := "not-found: /orgs/1/sec_policy/draft/ip_lists/7"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
