// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package datasources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRegexpStringValidator(t *testing.T) {
	t.Parallel()

	validResp := &validator.StringResponse{}
	(regexpStringValidator{}).ValidateString(
		context.Background(),
		validator.StringRequest{
			Path:        path.Root("name_pattern"),
			ConfigValue: types.StringValue(`^events-[0-9]+$`),
		},
		validResp,
	)
	if validResp.Diagnostics.HasError() {
		t.Fatalf("valid regular expression was rejected: %v", validResp.Diagnostics.Errors())
	}

	invalidResp := &validator.StringResponse{}
	(regexpStringValidator{}).ValidateString(
		context.Background(),
		validator.StringRequest{
			Path:        path.Root("name_pattern"),
			ConfigValue: types.StringValue(`[unclosed`),
		},
		invalidResp,
	)
	if !invalidResp.Diagnostics.HasError() {
		t.Fatal("invalid regular expression was accepted")
	}
}
