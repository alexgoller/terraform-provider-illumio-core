// Copyright 2021 Illumio, Inc. All Rights Reserved.

package client

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound is the sentinel for an HTTP 404 response.
//
// Use errors.Is(err, client.ErrNotFound) rather than matching on the message.
// Callers need to distinguish "the object is gone" from every other failure:
// a Terraform resource must remove a deleted object from state so the next
// plan recreates it, instead of failing the whole run.
var ErrNotFound = errors.New("not-found")

// ErrConflict is the sentinel for a 406 that means "this object already
// exists". Use errors.Is(err, client.ErrConflict).
//
// The PCE has no single token for this. These were observed on 26.30.2:
//
//	label_key_and_value_must_be_unique   creating a duplicate label
//	ip_list_name_not_unique              creating a duplicate IP list
//	name_must_be_unique                  creating a duplicate label group
//	rule_set_name_in_use                 creating a duplicate rule set
//
// Detection therefore matches the shape of the token rather than an exact
// list, so a token added by a later PCE version is still recognised. A token
// that is not recognised simply surfaces the error as before.
var ErrConflict = errors.New("already exists")

// NotAcceptableError reports an HTTP 406 from the PCE.
//
// Detail carries whatever the PCE said, and is what conflict detection
// inspects. Message overrides the rendered text when a caller needs to keep an
// existing user-facing string exactly as it was.
type NotAcceptableError struct {
	Detail  string
	Message string
}

func (e *NotAcceptableError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("not-acceptable: %v", e.Detail)
}

// Is reports ErrConflict when the PCE rejected the request because the object
// already exists.
func (e *NotAcceptableError) Is(target error) bool {
	if target != ErrConflict {
		return false
	}

	detail := strings.ToLower(e.Detail)
	for _, marker := range []string{"unique", "in_use", "in use", "already exists"} {
		if strings.Contains(detail, marker) {
			return true
		}
	}
	return false
}

// NotFoundError reports that a resource does not exist in the PCE.
type NotFoundError struct {
	Resource string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("not-found: %s", e.Resource)
}

// Is lets errors.Is(err, ErrNotFound) match this type.
func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}
