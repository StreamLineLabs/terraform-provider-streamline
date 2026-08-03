// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math"
	"net"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

func createSASLMechanism(mechanism, username, password string) (sasl.Mechanism, error) {
	switch mechanism {
	case "PLAIN":
		return plain.Mechanism{
			Username: username,
			Password: password,
		}, nil
	case "SCRAM-SHA-256":
		return scram.Mechanism(scram.SHA256, username, password)
	case "SCRAM-SHA-512":
		return scram.Mechanism(scram.SHA512, username, password)
	default:
		return nil, fmt.Errorf("unsupported SASL mechanism: %s", mechanism)
	}
}

func createTLSConfig(caCert, clientCert, clientKey string, skipVerify bool) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	tlsCfg.InsecureSkipVerify = skipVerify

	// Load custom CA certificate if provided
	if caCert != "" {
		// The CA certificate path comes from the provider's tls_ca_cert
		// attribute (or STREAMLINE_TLS_CA_CERT). Reading an operator-supplied
		// path is exactly the purpose of this option, so the variable file
		// path is intentional and scoped to this single read.
		caCertData, err := os.ReadFile(caCert) // #nosec G304 -- operator-configured CA certificate path
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate %s: %w", caCert, err)
		}
		caCertPool := tls.Config{}.RootCAs
		if caCertPool == nil {
			var certPoolErr error
			caCertPool, certPoolErr = x509.SystemCertPool()
			if certPoolErr != nil {
				caCertPool = x509.NewCertPool()
			}
		}
		if !caCertPool.AppendCertsFromPEM(caCertData) {
			return nil, fmt.Errorf("failed to parse CA certificate from %s", caCert)
		}
		tlsCfg.RootCAs = caCertPool
	}

	// Load client certificate and key for mTLS if provided
	if clientCert != "" && clientKey != "" {
		cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}

// withRetry executes fn with exponential backoff retry on transient errors.
// Base delay is 1s, doubling each attempt, capped at 10s.
func (c *StreamlineClient) withRetry(ctx context.Context, operation string, fn func(ctx context.Context) error) error {
	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		if attempt < c.maxRetries-1 {
			delay := time.Duration(math.Min(float64(time.Second)*math.Pow(2, float64(attempt)), float64(10*time.Second)))
			select {
			case <-ctx.Done():
				return fmt.Errorf("%s: %w (after %d attempts, context cancelled)", operation, lastErr, attempt+1)
			case <-time.After(delay):
			}
		}
	}
	return fmt.Errorf("%s: %w (after %d attempts)", operation, lastErr, c.maxRetries)
}

func (c *StreamlineClient) getControllerConn(ctx context.Context) (*kafka.Conn, error) {
	// Connect to any broker first
	conn, err := c.dialer.DialContext(ctx, "tcp", c.brokers[0])
	if err != nil {
		return nil, err
	}

	// Get the controller
	controller, err := conn.Controller()
	if err != nil {
		closeQuietly(ctx, conn, "broker connection")
		return nil, err
	}

	// If we're already connected to controller, return this connection
	if controller.Host == c.brokers[0] {
		return conn, nil
	}

	// Otherwise, connect to the controller
	closeQuietly(ctx, conn, "broker connection")
	return c.dialer.DialContext(ctx, "tcp", net.JoinHostPort(controller.Host, fmt.Sprintf("%d", controller.Port)))
}

// Close closes the client connections
func (c *StreamlineClient) Close() error {
	// kafka-go connections are closed individually
	return nil
}

// GracefulClose drains any pending operations and closes the client connection.
func (c *StreamlineClient) GracefulClose(ctx context.Context) error {
	// Signal shutdown intent
	c.mu.Lock()
	c.closing = true
	c.mu.Unlock()

	// Wait for in-flight requests to complete (with timeout from context)
	done := make(chan struct{})
	go func() {
		c.inflight.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All requests completed
	case <-ctx.Done():
		return fmt.Errorf("graceful shutdown timed out: %w", ctx.Err())
	}

	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	return nil
}
