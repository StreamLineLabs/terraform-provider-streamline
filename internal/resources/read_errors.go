// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

// handleReadError removes state only for a typed, confirmed not-found result.
// Every other error is reported and leaves the pre-populated response state
// untouched so transient, authorization, and protocol failures cannot cause
// Terraform to propose recreation.
func handleReadError(
	ctx context.Context,
	resp *resource.ReadResponse,
	resourceName string,
	identifier string,
	err error,
) bool {
	if err == nil {
		return false
	}
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return true
	}
	resp.Diagnostics.AddError(
		"Failed to Read "+resourceName,
		fmt.Sprintf("Unable to read %s %q: %s", resourceName, identifier, err),
	)
	return true
}

func handleDeleteError(
	respDiagnostics *resource.DeleteResponse,
	resourceName string,
	identifier string,
	err error,
) bool {
	if err == nil || client.IsNotFound(err) {
		return false
	}
	respDiagnostics.Diagnostics.AddError(
		"Failed to Delete "+resourceName,
		fmt.Sprintf("Unable to delete %s %q: %s", resourceName, identifier, err),
	)
	return true
}
