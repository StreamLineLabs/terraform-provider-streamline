// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// newSchemaRegistryTestServer starts an httptest.Server driven by handler and
// returns a SchemaRegistryClient configured with basic auth credentials that
// point at it. The server is closed automatically via t.Cleanup.
func newSchemaRegistryTestServer(t *testing.T, handler http.HandlerFunc) *SchemaRegistryClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewSchemaRegistryClient(SchemaRegistryConfig{
		URL:      srv.URL,
		Username: "user",
		Password: "pass",
	})
}

func TestSchemaRegistryClient_RegisterSchema(t *testing.T) {
	t.Parallel()

	var gotPath, gotMethod, gotContentType string
	var gotUser, gotPass string
	var gotAuthOK bool
	var gotBody registerSchemaRequest

	c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		gotUser, gotPass, gotAuthOK = r.BasicAuth()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(registerSchemaResponse{ID: 42})
	})

	id, err := c.RegisterSchema(context.Background(), SchemaConfig{
		Subject:    "orders-value",
		Schema:     `{"type":"record"}`,
		SchemaType: "AVRO",
		References: []SchemaReference{{Name: "ref1", Subject: "common", Version: 1}},
	})
	if err != nil {
		t.Fatalf("RegisterSchema() error = %v", err)
	}
	if id != 42 {
		t.Fatalf("id = %d, want 42", id)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/subjects/orders-value/versions" {
		t.Errorf("path = %q, want /subjects/orders-value/versions", gotPath)
	}
	if gotContentType != "application/vnd.schemaregistry.v1+json" {
		t.Errorf("content-type = %q", gotContentType)
	}
	if !gotAuthOK || gotUser != "user" || gotPass != "pass" {
		t.Errorf("basic auth = (%q, %q, %v), want (user, pass, true)", gotUser, gotPass, gotAuthOK)
	}
	if gotBody.Schema != `{"type":"record"}` {
		t.Errorf("body.Schema = %q", gotBody.Schema)
	}
	if gotBody.SchemaType != "AVRO" {
		t.Errorf("body.SchemaType = %q", gotBody.SchemaType)
	}
	if len(gotBody.References) != 1 || gotBody.References[0].Name != "ref1" ||
		gotBody.References[0].Subject != "common" || gotBody.References[0].Version != 1 {
		t.Errorf("body.References = %+v", gotBody.References)
	}
}

func TestSchemaRegistryClient_RegisterSchema_ErrorIncludesStatusAndBody(t *testing.T) {
	t.Parallel()

	c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error_code":42201,"message":"invalid schema"}`))
	})

	_, err := c.RegisterSchema(context.Background(), SchemaConfig{Subject: "s", Schema: "{}"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "422") {
		t.Errorf("error = %q, want it to contain status 422", err.Error())
	}
	if !strings.Contains(err.Error(), "invalid schema") {
		t.Errorf("error = %q, want it to contain response body", err.Error())
	}
}

func TestSchemaRegistryClient_GetSchema_Version(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		version    int
		wantPath   string
		wantResult int // version echoed back in the encoded response's Version field
	}{
		{name: "zero version uses latest", version: 0, wantPath: "/subjects/orders-value/versions/latest"},
		{name: "explicit version path", version: 3, wantPath: "/subjects/orders-value/versions/3"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotPath string
			c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				_ = json.NewEncoder(w).Encode(SchemaInfo{
					Subject: "orders-value",
					Version: tt.version,
					ID:      7,
					Schema:  `{"type":"record"}`,
				})
			})

			got, err := c.GetSchema(context.Background(), "orders-value", tt.version)
			if err != nil {
				t.Fatalf("GetSchema() error = %v", err)
			}
			if gotPath != tt.wantPath {
				t.Errorf("path = %q, want %q", gotPath, tt.wantPath)
			}
			if got.ID != 7 {
				t.Errorf("ID = %d, want 7", got.ID)
			}
		})
	}
}

func TestSchemaRegistryClient_GetSchema_NotFound(t *testing.T) {
	t.Parallel()

	c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := c.GetSchema(context.Background(), "orders-value", 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Fatalf("expected typed not-found, got %T: %v", err, err)
	}
	const want = "schema not found: orders-value version latest"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestSchemaRegistryClient_GetSchemaByID(t *testing.T) {
	t.Parallel()

	var gotPath string
	c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]string{"schema": `{"type":"record"}`})
	})

	schema, err := c.GetSchemaByID(context.Background(), 99)
	if err != nil {
		t.Fatalf("GetSchemaByID() error = %v", err)
	}
	if gotPath != "/schemas/ids/99" {
		t.Errorf("path = %q, want /schemas/ids/99", gotPath)
	}
	if schema != `{"type":"record"}` {
		t.Errorf("schema = %q", schema)
	}
}

func TestSchemaRegistryClient_GetSchemaVersionForID(t *testing.T) {
	t.Parallel()

	var gotPath string
	c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode([]SubjectVersionPair{
			{Subject: "other-value", Version: 9},
			{Subject: "orders-value", Version: 2},
			{Subject: "orders-value", Version: 4},
		})
	})

	version, err := c.GetSchemaVersionForID(context.Background(), "orders-value", 42)
	if err != nil {
		t.Fatalf("GetSchemaVersionForID() error = %v", err)
	}
	if gotPath != "/schemas/ids/42/versions" {
		t.Fatalf("path = %q, want /schemas/ids/42/versions", gotPath)
	}
	if version != 4 {
		t.Fatalf("version = %d, want highest matching version 4", version)
	}
}

func TestSchemaRegistryClient_GetSchemaVersionForIDSubjectMissing(t *testing.T) {
	t.Parallel()

	c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]SubjectVersionPair{{Subject: "other-value", Version: 1}})
	})

	_, err := c.GetSchemaVersionForID(context.Background(), "orders-value", 42)
	if !IsNotFound(err) {
		t.Fatalf("expected typed not-found, got %T: %v", err, err)
	}
}

func TestSchemaRegistryClient_DeleteSchema_PermanentQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		permanent    bool
		wantPath     string
		wantRawQuery string
	}{
		{name: "soft delete", permanent: false, wantPath: "/subjects/orders-value", wantRawQuery: ""},
		{name: "permanent delete", permanent: true, wantPath: "/subjects/orders-value", wantRawQuery: "permanent=true"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotPath, gotRawQuery, gotMethod string
			c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotRawQuery = r.URL.RawQuery
				gotMethod = r.Method
			})

			if err := c.DeleteSchema(context.Background(), "orders-value", tt.permanent); err != nil {
				t.Fatalf("DeleteSchema() error = %v", err)
			}
			if gotMethod != http.MethodDelete {
				t.Errorf("method = %q, want DELETE", gotMethod)
			}
			if gotPath != tt.wantPath {
				t.Errorf("path = %q, want %q", gotPath, tt.wantPath)
			}
			if gotRawQuery != tt.wantRawQuery {
				t.Errorf("rawQuery = %q, want %q", gotRawQuery, tt.wantRawQuery)
			}
		})
	}
}

func TestSchemaRegistryClient_SetCompatibility(t *testing.T) {
	t.Parallel()

	var gotPath, gotMethod string
	var gotBody map[string]string
	c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
	})

	if err := c.SetCompatibility(context.Background(), "orders-value", "FULL"); err != nil {
		t.Fatalf("SetCompatibility() error = %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/config/orders-value" {
		t.Errorf("path = %q, want /config/orders-value", gotPath)
	}
	if gotBody["compatibility"] != "FULL" {
		t.Errorf("body[compatibility] = %q, want FULL", gotBody["compatibility"])
	}
}

func TestSchemaRegistryClient_GetCompatibility(t *testing.T) {
	t.Parallel()

	c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(configResponse{CompatibilityLevel: "FORWARD"})
	})

	got, err := c.GetCompatibility(context.Background(), "orders-value")
	if err != nil {
		t.Fatalf("GetCompatibility() error = %v", err)
	}
	if got != "FORWARD" {
		t.Errorf("compatibility = %q, want FORWARD", got)
	}
}

func TestSchemaRegistryClient_GetCompatibility_SubjectNotFoundUsesGlobalConfig(t *testing.T) {
	t.Parallel()

	c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/config/orders-value":
			w.WriteHeader(http.StatusNotFound)
		case "/config":
			_ = json.NewEncoder(w).Encode(configResponse{CompatibilityLevel: "FULL"})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})

	got, err := c.GetCompatibility(context.Background(), "orders-value")
	if err != nil {
		t.Fatalf("GetCompatibility() error = %v", err)
	}
	if got != "FULL" {
		t.Errorf("compatibility = %q, want FULL", got)
	}
}

func TestSchemaRegistryClient_ListSubjects(t *testing.T) {
	t.Parallel()

	var gotPath, gotMethod string
	c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewEncoder(w).Encode([]string{"orders-value", "customers-value"})
	})

	got, err := c.ListSubjects(context.Background())
	if err != nil {
		t.Fatalf("ListSubjects() error = %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/subjects" {
		t.Errorf("path = %q, want /subjects", gotPath)
	}
	if len(got) != 2 || got[0] != "orders-value" || got[1] != "customers-value" {
		t.Errorf("subjects = %+v", got)
	}
}

func TestSchemaRegistryClient_CheckCompatibility_Success(t *testing.T) {
	t.Parallel()

	var gotPath, gotMethod string
	c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewEncoder(w).Encode(compatibilityResponse{IsCompatible: true})
	})

	got, err := c.CheckCompatibility(context.Background(), "orders-value", `{"type":"record"}`, "AVRO")
	if err != nil {
		t.Fatalf("CheckCompatibility() error = %v", err)
	}
	if !got {
		t.Error("compatible = false, want true")
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/compatibility/subjects/orders-value/versions/latest" {
		t.Errorf("path = %q, want /compatibility/subjects/orders-value/versions/latest", gotPath)
	}
}

func TestSchemaRegistryClient_CheckCompatibility_NotFoundMeansCompatible(t *testing.T) {
	t.Parallel()

	c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	got, err := c.CheckCompatibility(context.Background(), "orders-value", `{"type":"record"}`, "AVRO")
	if err != nil {
		t.Fatalf("CheckCompatibility() error = %v", err)
	}
	if !got {
		t.Error("compatible = false, want true for 404 (no existing schema)")
	}
}

func TestSchemaRegistryClientEscapesSubjectPathSegments(t *testing.T) {
	t.Parallel()

	subject := "orders/value?region=west#section %"
	escaped := url.PathEscape(subject)

	tests := []struct {
		name         string
		expectedPath string
		responseBody string
		call         func(*SchemaRegistryClient) error
	}{
		{
			name:         "register schema",
			expectedPath: "/subjects/" + escaped + "/versions",
			responseBody: `{"id":42}`,
			call: func(c *SchemaRegistryClient) error {
				_, err := c.RegisterSchema(context.Background(), SchemaConfig{
					Subject: subject,
					Schema:  `{"type":"record"}`,
				})
				return err
			},
		},
		{
			name:         "get schema",
			expectedPath: "/subjects/" + escaped + "/versions/3",
			responseBody: `{"subject":"ignored","version":3,"id":42,"schema":"{}"}`,
			call: func(c *SchemaRegistryClient) error {
				_, err := c.GetSchema(context.Background(), subject, 3)
				return err
			},
		},
		{
			name:         "delete schema",
			expectedPath: "/subjects/" + escaped,
			responseBody: `[]`,
			call: func(c *SchemaRegistryClient) error {
				return c.DeleteSchema(context.Background(), subject, false)
			},
		},
		{
			name:         "set compatibility",
			expectedPath: "/config/" + escaped,
			responseBody: `{}`,
			call: func(c *SchemaRegistryClient) error {
				return c.SetCompatibility(context.Background(), subject, "BACKWARD")
			},
		},
		{
			name:         "get compatibility",
			expectedPath: "/config/" + escaped,
			responseBody: `{"compatibilityLevel":"BACKWARD"}`,
			call: func(c *SchemaRegistryClient) error {
				_, err := c.GetCompatibility(context.Background(), subject)
				return err
			},
		},
		{
			name:         "check compatibility",
			expectedPath: "/compatibility/subjects/" + escaped + "/versions/latest",
			responseBody: `{"is_compatible":true}`,
			call: func(c *SchemaRegistryClient) error {
				_, err := c.CheckCompatibility(context.Background(), subject, "{}", "AVRO")
				return err
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotPath string
			c := newSchemaRegistryTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.EscapedPath()
				_, _ = w.Write([]byte(tt.responseBody))
			})

			if err := tt.call(c); err != nil {
				t.Fatalf("%s failed: %v", tt.name, err)
			}
			if gotPath != tt.expectedPath {
				t.Fatalf("escaped path = %q, want %q", gotPath, tt.expectedPath)
			}
		})
	}
}

func TestSchemaSubjectPathSegmentEscapesTraversalSegments(t *testing.T) {
	t.Parallel()

	if got := schemaSubjectPathSegment("."); got != "%2E" {
		t.Fatalf("single-dot subject = %q, want %%2E", got)
	}
	if got := schemaSubjectPathSegment(".."); got != "%2E%2E" {
		t.Fatalf("double-dot subject = %q, want %%2E%%2E", got)
	}
}

// TestSchemaRegistryClient_MalformedResponseBody verifies that a 200 response
// with a body that cannot be decoded into the expected type surfaces a decode
// error, for every method that decodes JSON from a successful response.
func TestSchemaRegistryClient_MalformedResponseBody(t *testing.T) {
	t.Parallel()

	malformed := func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not-valid-json`))
	}

	tests := []struct {
		name string
		call func(c *SchemaRegistryClient) error
	}{
		{
			name: "RegisterSchema",
			call: func(c *SchemaRegistryClient) error {
				_, err := c.RegisterSchema(context.Background(), SchemaConfig{Subject: "s", Schema: "{}"})
				return err
			},
		},
		{
			name: "GetSchema",
			call: func(c *SchemaRegistryClient) error {
				_, err := c.GetSchema(context.Background(), "s", 1)
				return err
			},
		},
		{
			name: "GetSchemaByID",
			call: func(c *SchemaRegistryClient) error {
				_, err := c.GetSchemaByID(context.Background(), 1)
				return err
			},
		},
		{
			name: "GetCompatibility",
			call: func(c *SchemaRegistryClient) error {
				_, err := c.GetCompatibility(context.Background(), "s")
				return err
			},
		},
		{
			name: "ListSubjects",
			call: func(c *SchemaRegistryClient) error {
				_, err := c.ListSubjects(context.Background())
				return err
			},
		},
		{
			name: "CheckCompatibility",
			call: func(c *SchemaRegistryClient) error {
				_, err := c.CheckCompatibility(context.Background(), "s", "{}", "AVRO")
				return err
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := newSchemaRegistryTestServer(t, malformed)
			err := tt.call(c)
			if err == nil {
				t.Fatal("expected decode error, got nil")
			}
			if !strings.Contains(err.Error(), "failed to decode response") {
				t.Errorf("error = %q, want it to mention decode failure", err.Error())
			}
		})
	}
}

// unreadableBody is an io.ReadCloser whose Read always fails, simulating a
// connection that terminates before any response body bytes are received
// (e.g. a truncated 404 response with no payload).
type unreadableBody struct {
	err error
}

func (b *unreadableBody) Read(_ []byte) (int, error) { return 0, b.err }
func (b *unreadableBody) Close() error               { return nil }

// statusOnlyRoundTripper is an http.RoundTripper stub that always returns a
// response with the configured status code whose body immediately fails to
// read with bodyErr. It is used to verify that operations which special-case
// a status code (e.g. 404) do not depend on successfully reading the
// response body to take that fast path.
type statusOnlyRoundTripper struct {
	statusCode int
	bodyErr    error
}

func (rt *statusOnlyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: rt.statusCode,
		Status:     http.StatusText(rt.statusCode),
		Body:       &unreadableBody{err: rt.bodyErr},
		Header:     make(http.Header),
	}, nil
}

// newSchemaRegistryClientWithTransport returns a SchemaRegistryClient whose
// underlying http.Client uses rt instead of talking to a real server.
func newSchemaRegistryClientWithTransport(rt http.RoundTripper) *SchemaRegistryClient {
	c := NewSchemaRegistryClient(SchemaRegistryConfig{URL: "http://schema-registry.invalid"})
	c.httpClient.Transport = rt
	return c
}

// TestSchemaRegistryClient_NotFoundFastPath_SurvivesUnreadableBody verifies
// that endpoint-specific 404 handling is based on the status code alone, even
// when the response body cannot be read.
func TestSchemaRegistryClient_NotFoundFastPath_SurvivesUnreadableBody(t *testing.T) {
	t.Parallel()

	rt := &statusOnlyRoundTripper{statusCode: http.StatusNotFound, bodyErr: io.ErrUnexpectedEOF}

	t.Run("GetSchema", func(t *testing.T) {
		t.Parallel()
		c := newSchemaRegistryClientWithTransport(rt)

		_, err := c.GetSchema(context.Background(), "orders-value", 0)
		if err == nil {
			t.Fatal("expected error")
		}
		const want = "schema not found: orders-value version latest"
		if err.Error() != want {
			t.Errorf("error = %q, want %q", err.Error(), want)
		}
	})

	t.Run("GetCompatibility", func(t *testing.T) {
		t.Parallel()
		c := newSchemaRegistryClientWithTransport(rt)

		_, err := c.GetCompatibility(context.Background(), "orders-value")
		if err == nil {
			t.Fatal("expected global compatibility lookup error")
		}
	})

	t.Run("CheckCompatibility", func(t *testing.T) {
		t.Parallel()
		c := newSchemaRegistryClientWithTransport(rt)

		got, err := c.CheckCompatibility(context.Background(), "orders-value", `{"type":"record"}`, "AVRO")
		if err != nil {
			t.Fatalf("CheckCompatibility() error = %v", err)
		}
		if !got {
			t.Error("compatible = false, want true for 404 (no existing schema)")
		}
	})
}

// TestSchemaRegistryClient_SuccessWithUnreadableBody verifies that a 200
// response whose body fails to read still surfaces an error, instead of
// being silently treated as success, for representative GET and POST
// operations that decode a body on success.
func TestSchemaRegistryClient_SuccessWithUnreadableBody(t *testing.T) {
	t.Parallel()

	rt := &statusOnlyRoundTripper{statusCode: http.StatusOK, bodyErr: io.ErrUnexpectedEOF}

	tests := []struct {
		name string
		call func(c *SchemaRegistryClient) error
	}{
		{
			name: "ListSubjects",
			call: func(c *SchemaRegistryClient) error {
				_, err := c.ListSubjects(context.Background())
				return err
			},
		},
		{
			name: "GetCompatibility",
			call: func(c *SchemaRegistryClient) error {
				_, err := c.GetCompatibility(context.Background(), "orders-value")
				return err
			},
		},
		{
			name: "CheckCompatibility",
			call: func(c *SchemaRegistryClient) error {
				_, err := c.CheckCompatibility(context.Background(), "orders-value", `{"type":"record"}`, "AVRO")
				return err
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newSchemaRegistryClientWithTransport(rt)

			err := tt.call(c)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "could not read response body") {
				t.Errorf("error = %q, want it to mention the unreadable body", err.Error())
			}
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Errorf("error = %v, want it to wrap io.ErrUnexpectedEOF", err)
			}
		})
	}
}
