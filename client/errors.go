// Copyright 2021 Illumio, Inc. All Rights Reserved.

package client

import (
	"errors"
	"fmt"
)

// ErrNotFound is the sentinel for an HTTP 404 response.
//
// Use errors.Is(err, client.ErrNotFound) rather than matching on the message.
// Callers need to distinguish "the object is gone" from every other failure:
// a Terraform resource must remove a deleted object from state so the next
// plan recreates it, instead of failing the whole run.
var ErrNotFound = errors.New("not-found")

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
