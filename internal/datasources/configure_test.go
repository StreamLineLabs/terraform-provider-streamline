// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package datasources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

type dataSourceConfigureCase struct {
	name string
	// newDataSource returns a fresh data source plus a func reporting whether
	// it captured the client it needs from the shared container.
	newDataSource func() (datasource.DataSourceWithConfigure, func() bool)
}

func dataSourceConfigureCases() []dataSourceConfigureCase {
	return []dataSourceConfigureCase{
		{
			name: "streamline_cluster",
			newDataSource: func() (datasource.DataSourceWithConfigure, func() bool) {
				d := &ClusterDataSource{}
				return d, func() bool { return d.kafkaClient != nil }
			},
		},
		{
			name: "streamline_topics",
			newDataSource: func() (datasource.DataSourceWithConfigure, func() bool) {
				d := &TopicsDataSource{}
				return d, func() bool { return d.kafkaClient != nil }
			},
		},
	}
}

func newTestClients(t *testing.T) *client.Clients {
	t.Helper()

	kafka, err := client.NewStreamlineClient(client.Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("NewStreamlineClient: %v", err)
	}
	return &client.Clients{Kafka: kafka}
}

func TestDataSourceConfigure_AcceptsSharedClients(t *testing.T) {
	clients := newTestClients(t)

	for _, tc := range dataSourceConfigureCases() {
		t.Run(tc.name, func(t *testing.T) {
			d, configured := tc.newDataSource()

			resp := &datasource.ConfigureResponse{}
			d.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: clients}, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics.Errors())
			}
			if !configured() {
				t.Fatal("Configure did not capture the Kafka client from *client.Clients")
			}
		})
	}
}

func TestDataSourceConfigure_RejectsUnexpectedType(t *testing.T) {
	clients := newTestClients(t)

	// A bare Kafka client is the wrong shape: the provider always publishes the
	// *client.Clients container.
	unexpected := []any{clients.Kafka, "not-a-client", struct{}{}}

	for _, tc := range dataSourceConfigureCases() {
		for _, providerData := range unexpected {
			t.Run(tc.name, func(t *testing.T) {
				d, configured := tc.newDataSource()

				resp := &datasource.ConfigureResponse{}
				d.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: providerData}, resp)

				if !resp.Diagnostics.HasError() {
					t.Fatalf("expected an error diagnostic for provider data %T", providerData)
				}
				if got := resp.Diagnostics.Errors()[0].Summary(); got != "Unexpected Data Source Configure Type" {
					t.Fatalf("unexpected diagnostic summary: %q", got)
				}
				if configured() {
					t.Fatal("Configure captured a client despite the type mismatch")
				}
			})
		}
	}
}

func TestDataSourceConfigure_NilProviderDataIsNoOp(t *testing.T) {
	for _, tc := range dataSourceConfigureCases() {
		t.Run(tc.name, func(t *testing.T) {
			d, configured := tc.newDataSource()

			resp := &datasource.ConfigureResponse{}
			d.Configure(context.Background(), datasource.ConfigureRequest{}, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics.Errors())
			}
			if configured() {
				t.Fatal("Configure captured a client from nil provider data")
			}
		})
	}
}
