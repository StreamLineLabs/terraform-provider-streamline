// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"testing"
	"time"
)

func TestNewMoonshotClientNormalizesConfiguration(t *testing.T) {
	t.Parallel()

	got := NewMoonshotClient(MoonshotConfig{
		BaseURL: "https://streamline.example/",
		Token:   "secret",
	})

	if got.base != "https://streamline.example" {
		t.Fatalf("base = %q, want trailing slash removed", got.base)
	}
	if got.token != "secret" {
		t.Fatalf("token = %q, want configured token", got.token)
	}
	if got.httpCli.Timeout != 10*time.Second {
		t.Fatalf("timeout = %s, want 10s default", got.httpCli.Timeout)
	}
}

func TestNewMoonshotClientPreservesPositiveTimeout(t *testing.T) {
	t.Parallel()

	got := NewMoonshotClient(MoonshotConfig{Timeout: 3 * time.Second})
	if got.httpCli.Timeout != 3*time.Second {
		t.Fatalf("timeout = %s, want 3s", got.httpCli.Timeout)
	}
}
