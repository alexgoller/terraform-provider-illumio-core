// Copyright 2021 Illumio, Inc. All Rights Reserved.

package client

import (
	"errors"
	"fmt"
	"testing"
)

// The tokens the PCE actually returns, observed on 26.30.2. They share no
// common substring, which is why detection matches their shape.
func TestNotAcceptableErrorIsConflict(t *testing.T) {
	conflicts := []string{
		"\nToken: label_key_and_value_must_be_unique\nMessage: Label key and value must be unique\n",
		"\nToken: ip_list_name_not_unique\nMessage: IP List name not unique\n",
		"\nToken: name_must_be_unique\nMessage: Name must be unique\n",
		"\nToken: rule_set_name_in_use\nMessage: Rule set name in use\n",
	}
	for _, detail := range conflicts {
		err := error(&NotAcceptableError{Detail: detail})
		if !errors.Is(err, ErrConflict) {
			t.Errorf("not recognised as a conflict: %s", detail)
		}
		if !errors.Is(fmt.Errorf("creating label: %w", err), ErrConflict) {
			t.Errorf("not recognised when wrapped: %s", detail)
		}
	}
}

// A 406 for any other reason must not be mistaken for "already exists" -
// adopting on an unrelated validation failure would silently manage the wrong
// object.
func TestNotAcceptableErrorNotAlwaysConflict(t *testing.T) {
	for _, detail := range []string{
		"\nToken: invalid_ip_address\nMessage: Invalid IP address specified\n",
		"\nToken: invalid_uri\nMessage: Invalid URI\n",
		"Token: unknown_property: Unknown property foo",
	} {
		if errors.Is(&NotAcceptableError{Detail: detail}, ErrConflict) {
			t.Errorf("wrongly treated as a conflict: %s", detail)
		}
	}
}

// The message is user-facing and must not change.
func TestNotAcceptableErrorMessageUnchanged(t *testing.T) {
	err := &NotAcceptableError{Detail: "some detail"}
	if got, want := err.Error(), "not-acceptable: some detail"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestOtherErrorsAreNotConflict(t *testing.T) {
	for _, err := range []error{
		errors.New("unauthorized: please check your credentials"),
		&NotFoundError{Resource: "/orgs/1/labels/1"},
	} {
		if errors.Is(err, ErrConflict) {
			t.Errorf("%v wrongly treated as a conflict", err)
		}
	}
}
