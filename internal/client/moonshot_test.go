// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *MoonshotClient) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewMoonshotClient(MoonshotConfig{BaseURL: srv.URL, Token: "tok"})
	return srv, c
}

func TestMoonshotClient_ListBranches(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/branches" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing bearer token")
		}
		_ = json.NewEncoder(w).Encode(listBranchesResponse{
			Branches: []Branch{{Name: "main", CreatedAtMs: 1}},
		})
	})
	got, err := c.ListBranches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "main" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestMoonshotClient_GetBranch_FoundAndMissing(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(listBranchesResponse{
			Branches: []Branch{{Name: "main"}, {Name: "exp", Parent: "main"}},
		})
	})
	b, err := c.GetBranch(context.Background(), "exp")
	if err != nil || b == nil || b.Parent != "main" {
		t.Fatalf("expected exp branch, got %+v err=%v", b, err)
	}
	missing, err := c.GetBranch(context.Background(), "nope")
	if err != nil || missing != nil {
		t.Fatalf("expected nil/nil, got %+v err=%v", missing, err)
	}
}

func TestMoonshotClient_CreateAndDeleteBranch(t *testing.T) {
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/branches":
			var in map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in["name"] != "exp" || in["parent"] != "main" {
				t.Errorf("bad body: %+v", in)
			}
			_ = json.NewEncoder(w).Encode(Branch{Name: "exp", Parent: "main", CreatedAtMs: 7})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/branches/exp":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	_ = srv
	b, err := c.CreateBranch(context.Background(), "exp", "main")
	if err != nil || b == nil || b.Name != "exp" {
		t.Fatalf("create: %+v %v", b, err)
	}
	if err := c.DeleteBranch(context.Background(), "exp"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestMoonshotClient_RegisterAndGetContract(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/contracts":
			_ = json.NewEncoder(w).Encode(Contract{
				Name:          "orders.v1",
				Schema:        map[string]interface{}{"type": "object"},
				Compatibility: "BACKWARD",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/contracts/orders.v1":
			_ = json.NewEncoder(w).Encode(Contract{
				Name:   "orders.v1",
				Schema: map[string]interface{}{"type": "object"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/contracts/missing":
			w.WriteHeader(http.StatusNotFound)
		}
	})

	got, err := c.RegisterContract(context.Background(), Contract{
		Name:          "orders.v1",
		Schema:        map[string]interface{}{"type": "object"},
		Compatibility: "BACKWARD",
	})
	if err != nil || got.Name != "orders.v1" {
		t.Fatalf("register: %+v %v", got, err)
	}
	g, err := c.GetContract(context.Background(), "orders.v1")
	if err != nil || g == nil || g.Name != "orders.v1" {
		t.Fatalf("get: %+v %v", g, err)
	}
	missing, err := c.GetContract(context.Background(), "missing")
	if err != nil || missing != nil {
		t.Fatalf("expected nil/nil for 404, got %+v %v", missing, err)
	}
}

func TestMoonshotClient_NonNotFoundErrorBubbles(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
	})
	_, err := c.ListBranches(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	me, ok := err.(*MoonshotError)
	if !ok || me.Status != http.StatusInternalServerError {
		t.Fatalf("expected MoonshotError 500, got %T %v", err, err)
	}
}
