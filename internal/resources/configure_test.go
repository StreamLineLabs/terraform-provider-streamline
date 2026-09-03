// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

// newTestClients builds the same container the provider publishes through
// resource.ConfigureRequest.ProviderData, with every optional client populated.
func newTestClients(t *testing.T) *client.Clients {
	t.Helper()

	kafka, err := client.NewStreamlineClient(client.Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("NewStreamlineClient: %v", err)
	}

	return &client.Clients{
		Kafka:          kafka,
		SchemaRegistry: client.NewSchemaRegistryClient(client.SchemaRegistryConfig{URL: "http://localhost:8081"}),
		Moonshot:       client.NewMoonshotClient(client.MoonshotConfig{BaseURL: "http://localhost:9094"}),
	}
}

// configureCase describes one resource's expectations around Configure.
type configureCase struct {
	name string
	// newResource returns a fresh resource plus a func reporting whether the
	// resource captured the client it needs from the shared container.
	newResource func() (resource.ResourceWithConfigure, func() bool)
}

func configureCases() []configureCase {
	return []configureCase{
		{
			name: "streamline_topic",
			newResource: func() (resource.ResourceWithConfigure, func() bool) {
				r := &TopicResource{}
				return r, func() bool { return r.kafkaClient != nil }
			},
		},
		{
			name: "streamline_acl",
			newResource: func() (resource.ResourceWithConfigure, func() bool) {
				r := &AclResource{}
				return r, func() bool { return r.kafkaClient != nil }
			},
		},
		{
			name: "streamline_schema",
			newResource: func() (resource.ResourceWithConfigure, func() bool) {
				r := &SchemaResource{}
				return r, func() bool { return r.schemaRegistryClient != nil }
			},
		},
		{
			name: "streamline_user",
			newResource: func() (resource.ResourceWithConfigure, func() bool) {
				r := &UserResource{}
				return r, func() bool { return r.configured }
			},
		},
		{
			name: "streamline_consumer_group",
			newResource: func() (resource.ResourceWithConfigure, func() bool) {
				r := &ConsumerGroupResource{}
				return r, func() bool { return r.kafkaClient != nil }
			},
		},
		{
			name: "streamline_branch",
			newResource: func() (resource.ResourceWithConfigure, func() bool) {
				r := &BranchResource{}
				return r, func() bool { return r.configured }
			},
		},
		{
			name: "streamline_contract",
			newResource: func() (resource.ResourceWithConfigure, func() bool) {
				r := &ContractResource{}
				return r, func() bool { return r.configured }
			},
		},
		{
			name: "streamline_memory",
			newResource: func() (resource.ResourceWithConfigure, func() bool) {
				r := &MemoryResource{}
				return r, func() bool { return r.configured }
			},
		},
	}
}

func TestResourceConfigure_AcceptsSharedClients(t *testing.T) {
	clients := newTestClients(t)

	for _, tc := range configureCases() {
		t.Run(tc.name, func(t *testing.T) {
			r, configured := tc.newResource()

			resp := &resource.ConfigureResponse{}
			r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: clients}, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics.Errors())
			}
			if !configured() {
				t.Fatal("Configure did not capture a client from *client.Clients")
			}
		})
	}
}

func TestResourceConfigure_RejectsUnexpectedType(t *testing.T) {
	kafka, err := client.NewStreamlineClient(client.Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("NewStreamlineClient: %v", err)
	}

	// A bare Kafka client is the wrong shape: the provider always publishes the
	// *client.Clients container.
	unexpected := []any{kafka, "not-a-client", struct{}{}}

	for _, tc := range configureCases() {
		for _, providerData := range unexpected {
			t.Run(tc.name, func(t *testing.T) {
				r, configured := tc.newResource()

				resp := &resource.ConfigureResponse{}
				r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: providerData}, resp)

				if !resp.Diagnostics.HasError() {
					t.Fatalf("expected an error diagnostic for provider data %T", providerData)
				}
				if got := resp.Diagnostics.Errors()[0].Summary(); got != "Unexpected Resource Configure Type" {
					t.Fatalf("unexpected diagnostic summary: %q", got)
				}
				if configured() {
					t.Fatal("Configure captured a client despite the type mismatch")
				}
			})
		}
	}
}

func TestResourceConfigure_NilProviderDataIsNoOp(t *testing.T) {
	for _, tc := range configureCases() {
		t.Run(tc.name, func(t *testing.T) {
			r, configured := tc.newResource()

			resp := &resource.ConfigureResponse{}
			r.Configure(context.Background(), resource.ConfigureRequest{}, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics.Errors())
			}
			if configured() {
				t.Fatal("Configure captured a client from nil provider data")
			}
		})
	}
}

func TestResourceConfigure_ReportsMissingOptionalClients(t *testing.T) {
	// Only the Kafka client is configured; schema registry backed resources
	// must explain what the practitioner has to set.
	kafka, err := client.NewStreamlineClient(client.Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("NewStreamlineClient: %v", err)
	}
	clients := &client.Clients{Kafka: kafka}

	cases := map[string]resource.ResourceWithConfigure{
		"streamline_schema": &SchemaResource{},
	}

	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			resp := &resource.ConfigureResponse{}
			r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: clients}, resp)

			if !resp.Diagnostics.HasError() {
				t.Fatal("expected an error diagnostic for the missing optional client")
			}
			if got := resp.Diagnostics.Errors()[0].Summary(); got == "Unexpected Resource Configure Type" {
				t.Fatalf("provider data was rejected as the wrong type: %q", got)
			}
		})
	}
}
