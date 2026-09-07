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
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

func (c *StreamlineClient) bootstrapAddr() net.Addr {
	return kafka.TCP(c.brokers...)
}

func (c *StreamlineClient) readClusterMetadata(
	ctx context.Context,
	topics []string,
) (*kafka.MetadataResponse, error) {
	resp, err := c.kafkaClient.Metadata(ctx, &kafka.MetadataRequest{
		Addr:   c.bootstrapAddr(),
		Topics: topics,
	})
	if err != nil {
		return nil, err
	}
	if err := validateMetadataResponse(resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func validateMetadataResponse(resp *kafka.MetadataResponse) error {
	if resp == nil {
		return fmt.Errorf("broker returned an empty metadata response")
	}
	if len(resp.Brokers) == 0 {
		return fmt.Errorf("metadata response advertised no brokers")
	}
	for i := range resp.Brokers {
		if _, err := kafkaBrokerAddr(&resp.Brokers[i]); err != nil {
			return fmt.Errorf("metadata response contained invalid broker %d: %w", i, err)
		}
	}
	return nil
}

func kafkaBrokerAddr(broker *kafka.Broker) (net.Addr, error) {
	host := strings.TrimSpace(broker.Host)
	if host == "" {
		return nil, fmt.Errorf("broker %d advertised an empty host", broker.ID)
	}
	if broker.Port <= 0 || broker.Port > 65535 {
		return nil, fmt.Errorf("broker %d advertised invalid port %d", broker.ID, broker.Port)
	}
	return kafka.TCP(net.JoinHostPort(host, strconv.Itoa(broker.Port))), nil
}

func (c *StreamlineClient) resolveControllerAddr(ctx context.Context) (net.Addr, error) {
	metadata, err := c.readClusterMetadata(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve controller metadata: %w", err)
	}

	controllerAddr, err := kafkaBrokerAddr(&metadata.Controller)
	if err != nil {
		return nil, fmt.Errorf("metadata response did not identify a valid controller: %w", err)
	}
	for i := range metadata.Brokers {
		broker := &metadata.Brokers[i]
		if broker.ID == metadata.Controller.ID &&
			broker.Host == metadata.Controller.Host &&
			broker.Port == metadata.Controller.Port {
			return controllerAddr, nil
		}
	}
	return nil, fmt.Errorf(
		"metadata response controller %d at %s was not present in the advertised broker list",
		metadata.Controller.ID,
		controllerAddr.String(),
	)
}

func (c *StreamlineClient) resolveGroupCoordinatorAddr(
	ctx context.Context,
	groupID string,
) (net.Addr, error) {
	resp, err := c.kafkaClient.FindCoordinator(ctx, &kafka.FindCoordinatorRequest{
		Addr:    c.bootstrapAddr(),
		Key:     groupID,
		KeyType: kafka.CoordinatorKeyTypeConsumer,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve coordinator for consumer group %q: %w", groupID, err)
	}
	if resp == nil {
		return nil, fmt.Errorf(
			"failed to resolve coordinator for consumer group %q: broker returned an empty response",
			groupID,
		)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf(
			"failed to resolve coordinator for consumer group %q: %w",
			groupID,
			resp.Error,
		)
	}
	if resp.Coordinator == nil {
		return nil, fmt.Errorf(
			"failed to resolve coordinator for consumer group %q: broker omitted the coordinator",
			groupID,
		)
	}
	return kafkaBrokerAddr(&kafka.Broker{
		ID:   resp.Coordinator.NodeID,
		Host: resp.Coordinator.Host,
		Port: resp.Coordinator.Port,
	})
}

func (c *StreamlineClient) advertisedBrokerAddrs(ctx context.Context) ([]net.Addr, error) {
	metadata, err := c.readClusterMetadata(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve advertised brokers: %w", err)
	}

	addresses := make([]net.Addr, 0, len(metadata.Brokers))
	for i := range metadata.Brokers {
		addr, err := kafkaBrokerAddr(&metadata.Brokers[i])
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, addr)
	}
	return addresses, nil
}

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
		if IsNotFound(lastErr) {
			return lastErr
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
