// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"testing"
)

func TestTrimTrailingHorizontalWhitespace(t *testing.T) {
	t.Parallel()

	input := []byte("first  \nsecond\t\nthird\n")
	want := []byte("first\nsecond\nthird\n")
	if got := trimTrailingHorizontalWhitespace(input); !bytes.Equal(got, want) {
		t.Fatalf("normalized Markdown = %q, want %q", got, want)
	}
}
