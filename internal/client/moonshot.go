// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

// Package client also provides MoonshotClient — a minimal HTTP client for the
// Streamline broker's Moonshot control plane (port 9094). Currently used by
// the streamline_branch and streamline_contract Terraform resources.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MoonshotConfig configures a MoonshotClient.
type MoonshotConfig struct {
	BaseURL string        // e.g. http://localhost:9094
	Token   string        // optional bearer token
	Timeout time.Duration // per-request timeout (default 10s)
}

// MoonshotClient is a thin HTTP client for the Streamline Moonshot APIs.
type MoonshotClient struct {
	base    string
	token   string
	httpCli *http.Client
}

// NewMoonshotClient builds a client.
func NewMoonshotClient(cfg MoonshotConfig) *MoonshotClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &MoonshotClient{
		base:    strings.TrimRight(cfg.BaseURL, "/"),
		token:   cfg.Token,
		httpCli: &http.Client{Timeout: timeout},
	}
}

// MoonshotError carries the HTTP status and body for a non-2xx response.
type MoonshotError struct {
	Status int
	Body   string
	Op     string
}

func (e *MoonshotError) Error() string {
	return fmt.Sprintf("%s: HTTP %d: %s", e.Op, e.Status, e.Body)
}

// Branch describes a Streamline branch.
type Branch struct {
	Name        string `json:"name"`
	Parent      string `json:"parent,omitempty"`
	CreatedAtMs int64  `json:"created_at_ms,omitempty"`
}

type listBranchesResponse struct {
	Branches []Branch `json:"branches"`
}

// Contract describes a registered contract.
type Contract struct {
	Name          string                 `json:"name"`
	Schema        map[string]interface{} `json:"schema"`
	Compatibility string                 `json:"compatibility,omitempty"`
}

// Memory describes an agent memory partition.
type Memory struct {
	AgentID           string       `json:"agent_id"`
	Tenant            string       `json:"tenant"`
	Tiers             MemoryTiers  `json:"tiers"`
	Decay             *MemoryDecay `json:"decay,omitempty"`
	EncryptionEnabled bool         `json:"encryption_enabled,omitempty"`
}

// MemoryTiers holds retention settings for each memory tier.
type MemoryTiers struct {
	EpisodicRetentionDays  int64 `json:"episodic_retention_days"`
	SemanticRetentionDays  int64 `json:"semantic_retention_days"`
	ProceduralRetentionDays int64 `json:"procedural_retention_days"`
}

// MemoryDecay holds decay configuration for automatic relevance scoring.
type MemoryDecay struct {
	HalfLifeDays float64 `json:"half_life_days"`
	Threshold    float64 `json:"threshold,omitempty"`
}

// ListBranches returns all branches.
func (c *MoonshotClient) ListBranches(ctx context.Context) ([]Branch, error) {
	var out listBranchesResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/branches", nil, &out); err != nil {
		return nil, err
	}
	return out.Branches, nil
}

// GetBranch fetches a single branch; returns nil and no error if not found.
func (c *MoonshotClient) GetBranch(ctx context.Context, name string) (*Branch, error) {
	branches, err := c.ListBranches(ctx)
	if err != nil {
		return nil, err
	}
	for i := range branches {
		if branches[i].Name == name {
			return &branches[i], nil
		}
	}
	return nil, nil
}

// CreateBranch creates a branch under the given parent (parent may be "").
func (c *MoonshotClient) CreateBranch(ctx context.Context, name, parent string) (*Branch, error) {
	body := map[string]interface{}{"name": name}
	if parent != "" {
		body["parent"] = parent
	}
	var out Branch
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/branches", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteBranch deletes a branch by name.
func (c *MoonshotClient) DeleteBranch(ctx context.Context, name string) error {
	path := "/api/v1/branches/" + url.PathEscape(name)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

// RegisterContract registers (or updates) a contract.
func (c *MoonshotClient) RegisterContract(ctx context.Context, contract Contract) (*Contract, error) {
	var out Contract
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/contracts", contract, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetContract returns a contract by name; returns nil and no error if not found (HTTP 404).
func (c *MoonshotClient) GetContract(ctx context.Context, name string) (*Contract, error) {
	path := "/api/v1/contracts/" + url.PathEscape(name)
	var out Contract
	err := c.doJSON(ctx, http.MethodGet, path, nil, &out)
	if err != nil {
		var me *MoonshotError
		if errAs(err, &me) && me.Status == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// DeleteContract removes a contract by name.
func (c *MoonshotClient) DeleteContract(ctx context.Context, name string) error {
	path := "/api/v1/contracts/" + url.PathEscape(name)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

// CreateMemory creates (or updates) an agent memory partition.
func (c *MoonshotClient) CreateMemory(ctx context.Context, memory Memory) (*Memory, error) {
	var out Memory
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/memories", memory, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMemory returns a memory partition; returns nil and no error if not found.
func (c *MoonshotClient) GetMemory(ctx context.Context, tenant, agentID string) (*Memory, error) {
	path := "/api/v1/memories/" + url.PathEscape(tenant) + "/" + url.PathEscape(agentID)
	var out Memory
	err := c.doJSON(ctx, http.MethodGet, path, nil, &out)
	if err != nil {
		var me *MoonshotError
		if errAs(err, &me) && me.Status == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// DeleteMemory removes an agent memory partition.
func (c *MoonshotClient) DeleteMemory(ctx context.Context, tenant, agentID string) error {
	path := "/api/v1/memories/" + url.PathEscape(tenant) + "/" + url.PathEscape(agentID)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

func (c *MoonshotClient) doJSON(ctx context.Context, method, path string, body, out interface{}) error {
	op := fmt.Sprintf("%s %s", method, path)
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%s: marshal body: %w", op, err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return fmt.Errorf("%s: build request: %w", op, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &MoonshotError{Status: resp.StatusCode, Body: string(respBody), Op: op}
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("%s: unmarshal: %w (body: %s)", op, err, string(respBody))
	}
	return nil
}

// errAs is a tiny wrapper around errors.As to avoid an import in clients.
func errAs(err error, target interface{}) bool {
	type asErr interface{ As(interface{}) bool }
	if a, ok := err.(asErr); ok {
		return a.As(target)
	}
	if me, ok := err.(*MoonshotError); ok {
		if t, ok := target.(**MoonshotError); ok {
			*t = me
			return true
		}
	}
	return false
}
