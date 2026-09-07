// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

// Clients is the container the provider publishes through
// resource.ConfigureRequest.ProviderData and datasource.ConfigureRequest.ProviderData.
//
// It lives in this package (rather than in provider, resources or datasources)
// so that every resource and data source can assert the exact same type. Any
// other placement forces duplicate declarations whose type identities differ,
// which makes the type assertion in Configure fail at runtime.
//
// SchemaRegistry and Moonshot are nil when the corresponding provider
// attributes are not configured; consumers must check before use.
type Clients struct {
	Kafka          *StreamlineClient
	SchemaRegistry *SchemaRegistryClient
	Moonshot       *MoonshotClient
}
