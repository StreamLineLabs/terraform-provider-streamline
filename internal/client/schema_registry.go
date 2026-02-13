// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SchemaRegistryClient provides methods for interacting with Schema Registry
type SchemaRegistryClient struct {
	baseURL    string
	httpClient *http.Client
	username   string
	password   string
}

// SchemaRegistryConfig holds configuration for Schema Registry client
type SchemaRegistryConfig struct {
	URL      string
	Username string
	Password string
	Timeout  time.Duration
}

// NewSchemaRegistryClient creates a new Schema Registry client
func NewSchemaRegistryClient(cfg SchemaRegistryConfig) *SchemaRegistryClient {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &SchemaRegistryClient{
		baseURL: cfg.URL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		username: cfg.Username,
		password: cfg.Password,
	}
}

// SchemaConfig represents schema configuration
type SchemaConfig struct {
	Subject       string
	Schema        string
	SchemaType    string // AVRO, JSON, PROTOBUF
	References    []SchemaReference
	Compatibility string
}

// SchemaReference represents a schema reference
type SchemaReference struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Version int    `json:"version"`
}

// SchemaInfo represents schema information returned from registry
type SchemaInfo struct {
	Subject    string            `json:"subject"`
	Version    int               `json:"version"`
	ID         int               `json:"id"`
	Schema     string            `json:"schema"`
	SchemaType string            `json:"schemaType"`
	References []SchemaReference `json:"references,omitempty"`
}

// RegisterSchemaRequest represents the request to register a schema
type registerSchemaRequest struct {
	Schema     string            `json:"schema"`
	SchemaType string            `json:"schemaType,omitempty"`
	References []SchemaReference `json:"references,omitempty"`
}

// RegisterSchemaResponse represents the response from registering a schema
type registerSchemaResponse struct {
	ID int `json:"id"`
}

// CompatibilityResponse represents compatibility check response
type compatibilityResponse struct {
	IsCompatible bool `json:"is_compatible"`
}

// ConfigResponse represents compatibility config response
type configResponse struct {
	CompatibilityLevel string `json:"compatibilityLevel"`
}

// RegisterSchema registers a new schema version
func (c *SchemaRegistryClient) RegisterSchema(ctx context.Context, cfg SchemaConfig) (int, error) {
	url := fmt.Sprintf("%s/subjects/%s/versions", c.baseURL, cfg.Subject)

	reqBody := registerSchemaRequest{
		Schema:     cfg.Schema,
		SchemaType: cfg.SchemaType,
		References: cfg.References,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to register schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to register schema: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result registerSchemaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.ID, nil
}

// GetSchema retrieves a schema by subject and version
func (c *SchemaRegistryClient) GetSchema(ctx context.Context, subject string, version int) (*SchemaInfo, error) {
	versionStr := "latest"
	if version > 0 {
		versionStr = fmt.Sprintf("%d", version)
	}

	url := fmt.Sprintf("%s/subjects/%s/versions/%s", c.baseURL, subject, versionStr)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("schema not found: %s version %s", subject, versionStr)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get schema: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result SchemaInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetSchemaByID retrieves a schema by its global ID
func (c *SchemaRegistryClient) GetSchemaByID(ctx context.Context, id int) (string, error) {
	url := fmt.Sprintf("%s/schemas/ids/%d", c.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get schema: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Schema string `json:"schema"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Schema, nil
}

// DeleteSchema deletes a schema subject (soft delete)
func (c *SchemaRegistryClient) DeleteSchema(ctx context.Context, subject string, permanent bool) error {
	url := fmt.Sprintf("%s/subjects/%s", c.baseURL, subject)
	if permanent {
		url += "?permanent=true"
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete schema: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// SetCompatibility sets the compatibility level for a subject
func (c *SchemaRegistryClient) SetCompatibility(ctx context.Context, subject, level string) error {
	url := fmt.Sprintf("%s/config/%s", c.baseURL, subject)

	reqBody := map[string]string{
		"compatibility": level,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set compatibility: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set compatibility: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetCompatibility gets the compatibility level for a subject
func (c *SchemaRegistryClient) GetCompatibility(ctx context.Context, subject string) (string, error) {
	url := fmt.Sprintf("%s/config/%s", c.baseURL, subject)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get compatibility: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Return global default
		return "BACKWARD", nil
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get compatibility: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result configResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.CompatibilityLevel, nil
}

// ListSubjects lists all subjects in the registry
func (c *SchemaRegistryClient) ListSubjects(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/subjects", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list subjects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list subjects: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var subjects []string
	if err := json.NewDecoder(resp.Body).Decode(&subjects); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return subjects, nil
}

// CheckCompatibility checks if a schema is compatible
func (c *SchemaRegistryClient) CheckCompatibility(ctx context.Context, subject, schema, schemaType string) (bool, error) {
	url := fmt.Sprintf("%s/compatibility/subjects/%s/versions/latest", c.baseURL, subject)

	reqBody := registerSchemaRequest{
		Schema:     schema,
		SchemaType: schemaType,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return false, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to check compatibility: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No existing schema, so any schema is compatible
		return true, nil
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("failed to check compatibility: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result compatibilityResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.IsCompatible, nil
}

func (c *SchemaRegistryClient) setAuth(req *http.Request) {
	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
}
