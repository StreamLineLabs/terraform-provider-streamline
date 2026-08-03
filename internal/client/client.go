// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
)

// StreamlineClient provides methods for interacting with Streamline/Kafka clusters
type StreamlineClient struct {
	brokers     []string
	dialer      *kafka.Dialer
	kafkaClient *kafka.Client
	tlsConfig   *tls.Config
	sasl        sasl.Mechanism
	timeout     time.Duration
	maxRetries  int
	mu          sync.RWMutex
	closing     bool
	inflight    sync.WaitGroup
	httpClient  *http.Client
}

// Config holds configuration for creating a StreamlineClient via NewStreamlineClient.
type Config struct {
	Brokers           []string
	ConnectionTimeout time.Duration
	Timeout           time.Duration
	MaxRetries        int
	TLSEnabled        bool
	TLSCACertPath     string
	TLSCertPath       string
	TLSKeyPath        string
	TLSSkipVerify     bool
	SASLMechanism     string
	SASLUsername      string
	SASLPassword      string
}

// NewStreamlineClient creates a new StreamlineClient with the given configuration.
func NewStreamlineClient(cfg Config) (*StreamlineClient, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("no bootstrap servers provided")
	}

	client := &StreamlineClient{
		brokers:    cfg.Brokers,
		timeout:    cfg.Timeout,
		maxRetries: cfg.MaxRetries,
	}

	if client.timeout == 0 {
		client.timeout = 30 * time.Second
	}
	connectionTimeout := cfg.ConnectionTimeout
	if connectionTimeout == 0 {
		connectionTimeout = client.timeout
	}
	client.httpClient = &http.Client{Timeout: client.timeout}

	if client.maxRetries <= 0 {
		client.maxRetries = 3
	}

	// Configure SASL if provided
	if cfg.SASLMechanism != "" {
		mechanism, err := createSASLMechanism(cfg.SASLMechanism, cfg.SASLUsername, cfg.SASLPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to create SASL mechanism: %w", err)
		}
		client.sasl = mechanism
	}

	// Configure TLS if enabled
	if cfg.TLSEnabled {
		tlsCfg, err := createTLSConfig(cfg.TLSCACertPath, cfg.TLSCertPath, cfg.TLSKeyPath, cfg.TLSSkipVerify)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
		client.tlsConfig = tlsCfg
	}

	// Create dialer
	client.dialer = &kafka.Dialer{
		Timeout:       connectionTimeout,
		DualStack:     true,
		SASLMechanism: client.sasl,
		TLS:           client.tlsConfig,
	}

	// Create kafka.Client for admin operations (ACLs, partitions)
	transport := &kafka.Transport{
		Dial: client.dialer.DialFunc,
		TLS:  client.tlsConfig,
		SASL: client.sasl,
	}
	client.kafkaClient = &kafka.Client{
		Addr:      kafka.TCP(cfg.Brokers[0]),
		Timeout:   client.timeout,
		Transport: transport,
	}

	return client, nil
}
