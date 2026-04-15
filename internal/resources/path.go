// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import "github.com/hashicorp/terraform-plugin-framework/path"

// pathRoot is a tiny helper so we don't need to import the framework's path
// package in every resource file.
func pathRoot(name string) path.Path { return path.Root(name) }
