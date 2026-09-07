// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"io"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// closeQuietly closes c and records a failure at debug level.
//
// It is used on deferred cleanup paths where the close error cannot change the
// outcome of an operation that has already completed, but where silently
// dropping it would hide connection and file-descriptor leaks.
func closeQuietly(ctx context.Context, c io.Closer, what string) {
	if err := c.Close(); err != nil {
		tflog.Debug(ctx, "failed to close "+what, map[string]any{
			"error": err.Error(),
		})
	}
}
