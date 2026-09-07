// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

func addUnsupportedMoonshotResourceDiagnostic(diags *diag.Diagnostics, resourceName string) {
	diags.AddError(
		"Moonshot Resource Unsupported",
		resourceName+" is retained only so legacy state remains decodable. "+
			"Its legacy Terraform schema does not represent a provisionable object in the current broker API. "+
			"Do not use it in new configurations; remove legacy state only after verifying the external system of record.",
	)
}

func addUnsupportedMoonshotReadWarning(diags *diag.Diagnostics, resourceName string) {
	diags.AddWarning(
		"Legacy Moonshot State Cannot Be Refreshed",
		resourceName+" does not map to a provisionable object in the current broker API. "+
			"Terraform is preserving the legacy state unchanged without claiming remote verification.",
	)
}

func configureLegacyResource(req resource.ConfigureRequest, resp *resource.ConfigureResponse) bool {
	if req.ProviderData == nil {
		return false
	}
	if _, ok := req.ProviderData.(*client.Clients); !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Clients, got: %T", req.ProviderData),
		)
		return false
	}
	return true
}
