// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsNotFoundRequiresTypedError(t *testing.T) {
	t.Parallel()

	cause := errors.New("remote object is absent")
	notFound := NewNotFoundError("topic", "events", cause)
	if !IsNotFound(notFound) {
		t.Fatal("typed not-found error was not recognized")
	}
	if !IsNotFound(fmt.Errorf("read failed: %w", notFound)) {
		t.Fatal("wrapped typed not-found error was not recognized")
	}
	if !errors.Is(notFound, cause) {
		t.Fatal("not-found error did not preserve its cause")
	}

	for _, err := range []error{
		errors.New("topic not found: events"),
		errors.New("authorization failed"),
		errors.New("connection reset"),
	} {
		if IsNotFound(err) {
			t.Fatalf("untyped error was incorrectly classified as not-found: %v", err)
		}
	}
}
