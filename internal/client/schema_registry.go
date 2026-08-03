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

	resp, err := c.send(ctx, http.MethodPost, url, "register schema", reqBody)
	if resp == nil {
		return 0, err
	}
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, responseError("register schema", resp.StatusCode, resp.Body)
	}

	var result registerSchemaResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
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

	resp, err := c.send(ctx, http.MethodGet, url, "get schema", nil)
	if resp == nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("schema not found: %s version %s", subject, versionStr)
	}

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, responseError("get schema", resp.StatusCode, resp.Body)
	}

	var result SchemaInfo
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetSchemaByID retrieves a schema by its global ID
func (c *SchemaRegistryClient) GetSchemaByID(ctx context.Context, id int) (string, error) {
	url := fmt.Sprintf("%s/schemas/ids/%d", c.baseURL, id)

	resp, err := c.send(ctx, http.MethodGet, url, "get schema", nil)
	if resp == nil {
		return "", err
	}
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", responseError("get schema", resp.StatusCode, resp.Body)
	}

	var result struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
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

	resp, err := c.send(ctx, http.MethodDelete, url, "delete schema", nil)
	if resp == nil {
		return err
	}
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return responseError("delete schema", resp.StatusCode, resp.Body)
	}

	return nil
}

// SetCompatibility sets the compatibility level for a subject
func (c *SchemaRegistryClient) SetCompatibility(ctx context.Context, subject, level string) error {
	url := fmt.Sprintf("%s/config/%s", c.baseURL, subject)

	reqBody := map[string]string{
		"compatibility": level,
	}

	resp, err := c.send(ctx, http.MethodPut, url, "set compatibility", reqBody)
	if resp == nil {
		return err
	}
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return responseError("set compatibility", resp.StatusCode, resp.Body)
	}

	return nil
}

// GetCompatibility gets the compatibility level for a subject
func (c *SchemaRegistryClient) GetCompatibility(ctx context.Context, subject string) (string, error) {
	url := fmt.Sprintf("%s/config/%s", c.baseURL, subject)

	resp, err := c.send(ctx, http.MethodGet, url, "get compatibility", nil)
	if resp == nil {
		return "", err
	}

	if resp.StatusCode == http.StatusNotFound {
		// Return global default
		return "BACKWARD", nil
	}

	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", responseError("get compatibility", resp.StatusCode, resp.Body)
	}

	var result configResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.CompatibilityLevel, nil
}

// ListSubjects lists all subjects in the registry
func (c *SchemaRegistryClient) ListSubjects(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/subjects", c.baseURL)

	resp, err := c.send(ctx, http.MethodGet, url, "list subjects", nil)
	if resp == nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, responseError("list subjects", resp.StatusCode, resp.Body)
	}

	var subjects []string
	if err := json.Unmarshal(resp.Body, &subjects); err != nil {
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

	resp, err := c.send(ctx, http.MethodPost, url, "check compatibility", reqBody)
	if resp == nil {
		return false, err
	}

	if resp.StatusCode == http.StatusNotFound {
		// No existing schema, so any schema is compatible
		return true, nil
	}

	if err != nil {
		return false, err
	}

	if resp.StatusCode != http.StatusOK {
		return false, responseError("check compatibility", resp.StatusCode, resp.Body)
	}

	var result compatibilityResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.IsCompatible, nil
}

func (c *SchemaRegistryClient) setAuth(req *http.Request) {
	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
}

// schemaRegistryResponse is the outcome of a completed Schema Registry
// request: the HTTP status code and whatever portion of the response body
// could be read. Each public operation keeps its own endpoint-specific
// policy (which statuses are expected, how a 404 should be interpreted, and
// how the body decodes). StatusCode is always populated once a response was
// received from the server, even if reading the body subsequently failed,
// so callers can still act on status-only fast paths (e.g. 404) without
// needing the body.
type schemaRegistryResponse struct {
	StatusCode int
	Body       []byte
}

// send is the transport helper shared by every Schema Registry operation. It
// owns the mechanics common to all requests: marshaling an optional JSON
// body, building the request with context, setting the Accept/Content-Type
// headers the caller needs, applying Basic Auth, executing the request, and
// closing/reading the response body. reqBody may be nil for requests
// without a body. Headers follow each endpoint's original behavior: a JSON
// body sets Content-Type, a bodyless GET sets Accept, and a bodyless
// non-GET (e.g. DELETE) sets neither. op names the operation for error
// messages (e.g. "get schema").
//
// The returned response is non-nil whenever a response was received from the
// server, even if the returned error is non-nil because the body could not
// be fully read; StatusCode is always valid in that case, so callers must
// check status-only fast paths (like 404) before consulting the error or the
// body. A nil response indicates the request itself failed (e.g. could not
// be built or sent), in which case the error should be returned as-is.
func (c *SchemaRegistryClient) send(ctx context.Context, method, url, op string, reqBody interface{}) (*schemaRegistryResponse, error) {
	var bodyReader io.Reader = http.NoBody
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")
	} else if method == http.MethodGet {
		req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to %s: %w", op, err)
	}
	defer closeQuietly(ctx, resp.Body, "schema registry response body")

	respBody, readErr := io.ReadAll(resp.Body)
	result := &schemaRegistryResponse{StatusCode: resp.StatusCode, Body: respBody}
	if readErr != nil {
		return result, fmt.Errorf("failed to %s: status %d, could not read response body: %w", op, resp.StatusCode, readErr)
	}

	return result, nil
}

// responseError builds an error for a non-success Schema Registry response.
// The registry encodes its error_code/message payload in the body, so the
// body is included verbatim.
func responseError(op string, statusCode int, body []byte) error {
	return fmt.Errorf("failed to %s: status %d, body: %s", op, statusCode, string(body))
}
