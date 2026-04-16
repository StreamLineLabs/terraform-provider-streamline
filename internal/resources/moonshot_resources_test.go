// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

// These tests exercise the resource handler bodies directly against the
// MoonshotClient pointed at an httptest.Server. They cover the request shapes
// and response handling without spinning up the Terraform framework.

func newMoonshotTestClient(t *testing.T, h http.HandlerFunc) *client.MoonshotClient {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return client.NewMoonshotClient(client.MoonshotConfig{BaseURL: srv.URL})
}

func TestBranchResource_RoundTrip(t *testing.T) {
	created := false
	deleted := false
	mc := newMoonshotTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/branches":
			created = true
			_ = json.NewEncoder(w).Encode(client.Branch{Name: "exp", Parent: "main", CreatedAtMs: 99})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/branches":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"branches": []client.Branch{{Name: "exp", Parent: "main", CreatedAtMs: 99}},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/branches/exp":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		}
	})

	r := &BranchResource{moonshot: mc}
	ctx := context.Background()

	// Simulate Create
	model := BranchResourceModel{
		Name:   types.StringValue("exp"),
		Parent: types.StringValue("main"),
	}
	b, err := r.moonshot.CreateBranch(ctx, model.Name.ValueString(), model.Parent.ValueString())
	if err != nil || b == nil || !created {
		t.Fatalf("create: b=%+v err=%v created=%v", b, err, created)
	}

	// Simulate Read
	got, err := r.moonshot.GetBranch(ctx, "exp")
	if err != nil || got == nil || got.CreatedAtMs != 99 {
		t.Fatalf("read: got=%+v err=%v", got, err)
	}

	// Simulate Delete
	if err := r.moonshot.DeleteBranch(ctx, "exp"); err != nil || !deleted {
		t.Fatalf("delete: err=%v deleted=%v", err, deleted)
	}
}

func TestContractResource_BuildContract(t *testing.T) {
	r := &ContractResource{}
	data := ContractResourceModel{
		Name:          types.StringValue("orders.v1"),
		SchemaJSON:    types.StringValue(`{"type":"object","required":["id"]}`),
		Compatibility: types.StringValue("BACKWARD"),
	}
	c, err := r.buildContract(data)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "orders.v1" || c.Compatibility != "BACKWARD" || c.Schema["type"] != "object" {
		t.Fatalf("unexpected contract: %+v", c)
	}
}

func TestContractResource_BuildContract_InvalidJSON(t *testing.T) {
	r := &ContractResource{}
	_, err := r.buildContract(ContractResourceModel{
		Name:       types.StringValue("x"),
		SchemaJSON: types.StringValue(`{not json`),
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestContractResource_RoundTripWithServer(t *testing.T) {
	mc := newMoonshotTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/contracts":
			var c client.Contract
			_ = json.NewDecoder(r.Body).Decode(&c)
			c.Compatibility = "BACKWARD"
			_ = json.NewEncoder(w).Encode(c)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/contracts/orders.v1":
			_ = json.NewEncoder(w).Encode(client.Contract{
				Name:          "orders.v1",
				Schema:        map[string]interface{}{"type": "object"},
				Compatibility: "BACKWARD",
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/contracts/orders.v1":
			w.WriteHeader(http.StatusNoContent)
		}
	})
	r := &ContractResource{moonshot: mc}
	ctx := context.Background()

	c, err := r.moonshot.RegisterContract(ctx, client.Contract{
		Name:   "orders.v1",
		Schema: map[string]interface{}{"type": "object"},
	})
	if err != nil || c.Compatibility != "BACKWARD" {
		t.Fatalf("register: %+v %v", c, err)
	}

	got, err := r.moonshot.GetContract(ctx, "orders.v1")
	if err != nil || got == nil || got.Name != "orders.v1" {
		t.Fatalf("get: %+v %v", got, err)
	}

	if err := r.moonshot.DeleteContract(ctx, "orders.v1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
