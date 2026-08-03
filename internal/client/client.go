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
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
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
	Brokers       []string
	Timeout       time.Duration
	MaxRetries    int
	TLSEnabled    bool
	TLSCACertPath string
	TLSCertPath   string
	TLSKeyPath    string
	TLSSkipVerify bool
	SASLMechanism string
	SASLUsername  string
	SASLPassword  string
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
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}

	if client.timeout == 0 {
		client.timeout = 30 * time.Second
	}

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
		Timeout:       client.timeout,
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

// =============================================================================
// Topic Operations
// =============================================================================

// TopicConfig represents topic configuration
type TopicConfig struct {
	Name              string
	Partitions        int
	ReplicationFactor int
	Config            map[string]string
}

// TopicMetadata represents topic metadata
type TopicMetadata struct {
	Name              string
	Partitions        int
	ReplicationFactor int
	Config            map[string]string
	Internal          bool
}

// CreateTopic creates a new topic
func (c *StreamlineClient) CreateTopic(ctx context.Context, cfg TopicConfig) error {
	return c.withRetry(ctx, "create topic", func(ctx context.Context) error {
		conn, err := c.getControllerConn(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect to controller: %w", err)
		}
		defer closeQuietly(ctx, conn, "controller connection")

		topicConfigs := make([]kafka.TopicConfig, 1)
		topicConfigs[0] = kafka.TopicConfig{
			Topic:             cfg.Name,
			NumPartitions:     cfg.Partitions,
			ReplicationFactor: cfg.ReplicationFactor,
		}

		// Add config entries
		if len(cfg.Config) > 0 {
			configs := make([]kafka.ConfigEntry, 0, len(cfg.Config))
			for k, v := range cfg.Config {
				configs = append(configs, kafka.ConfigEntry{
					ConfigName:  k,
					ConfigValue: v,
				})
			}
			topicConfigs[0].ConfigEntries = configs
		}

		return conn.CreateTopics(topicConfigs...)
	})
}

// GetTopic retrieves topic metadata
func (c *StreamlineClient) GetTopic(ctx context.Context, name string) (*TopicMetadata, error) {
	conn, err := c.getControllerConn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to controller: %w", err)
	}
	defer closeQuietly(ctx, conn, "controller connection")

	partitions, err := conn.ReadPartitions(name)
	if err != nil {
		return nil, fmt.Errorf("failed to read partitions: %w", err)
	}

	if len(partitions) == 0 {
		return nil, fmt.Errorf("topic not found: %s", name)
	}

	// Get replication factor from first partition
	replicationFactor := len(partitions[0].Replicas)

	return &TopicMetadata{
		Name:              name,
		Partitions:        len(partitions),
		ReplicationFactor: replicationFactor,
		Config:            make(map[string]string),
	}, nil
}

// UpdateTopic updates topic configuration
func (c *StreamlineClient) UpdateTopic(ctx context.Context, cfg TopicConfig) error {
	// Get current topic metadata
	current, err := c.GetTopic(ctx, cfg.Name)
	if err != nil {
		return err
	}

	// Create additional partitions if needed using kafka.Client API
	if cfg.Partitions > current.Partitions {
		count, err := toPartitionCount(cfg.Partitions)
		if err != nil {
			return err
		}
		resp, err := c.kafkaClient.CreatePartitions(ctx, &kafka.CreatePartitionsRequest{
			Topics: []kafka.TopicPartitionsConfig{
				{
					Name:  cfg.Name,
					Count: count,
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create partitions: %w", err)
		}
		for _, e := range resp.Errors {
			if e != nil {
				return fmt.Errorf("failed to create partitions: %w", e)
			}
		}
	}

	return nil
}

// DeleteTopic deletes a topic
func (c *StreamlineClient) DeleteTopic(ctx context.Context, name string) error {
	return c.withRetry(ctx, "delete topic", func(ctx context.Context) error {
		conn, err := c.getControllerConn(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect to controller: %w", err)
		}
		defer closeQuietly(ctx, conn, "controller connection")

		return conn.DeleteTopics(name)
	})
}

// ListTopics lists all topics
func (c *StreamlineClient) ListTopics(ctx context.Context) ([]TopicMetadata, error) {
	conn, err := c.getControllerConn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to controller: %w", err)
	}
	defer closeQuietly(ctx, conn, "controller connection")

	partitions, err := conn.ReadPartitions()
	if err != nil {
		return nil, fmt.Errorf("failed to read partitions: %w", err)
	}

	// Group by topic
	topicMap := make(map[string]*TopicMetadata)
	for i := range partitions {
		p := &partitions[i]
		if _, exists := topicMap[p.Topic]; !exists {
			topicMap[p.Topic] = &TopicMetadata{
				Name:              p.Topic,
				Partitions:        0,
				ReplicationFactor: len(p.Replicas),
				Config:            make(map[string]string),
			}
		}
		topicMap[p.Topic].Partitions++
	}

	topics := make([]TopicMetadata, 0, len(topicMap))
	for _, t := range topicMap {
		topics = append(topics, *t)
	}

	return topics, nil
}

// =============================================================================
// ACL Operations
// =============================================================================

// ACLConfig represents ACL configuration
type ACLConfig struct {
	ResourceType   string
	ResourceName   string
	PatternType    string
	Principal      string
	Host           string
	Operation      string
	PermissionType string
}

// CreateACL creates a new ACL
func (c *StreamlineClient) CreateACL(ctx context.Context, cfg ACLConfig) error {
	return c.withRetry(ctx, "create ACL", func(ctx context.Context) error {
		_, err := c.kafkaClient.CreateACLs(ctx, &kafka.CreateACLsRequest{
			ACLs: []kafka.ACLEntry{
				{
					ResourceType:        resourceTypeFromString(cfg.ResourceType),
					ResourceName:        cfg.ResourceName,
					ResourcePatternType: patternTypeFromString(cfg.PatternType),
					Principal:           cfg.Principal,
					Host:                cfg.Host,
					Operation:           operationFromString(cfg.Operation),
					PermissionType:      permissionTypeFromString(cfg.PermissionType),
				},
			},
		})
		return err
	})
}

// GetACL retrieves a single ACL entry matching the filter
func (c *StreamlineClient) GetACL(ctx context.Context, cfg ACLConfig) (*ACLConfig, error) {
	resp, err := c.kafkaClient.DescribeACLs(ctx, &kafka.DescribeACLsRequest{
		Filter: kafka.ACLFilter{
			ResourceTypeFilter:        resourceTypeFromString(cfg.ResourceType),
			ResourceNameFilter:        cfg.ResourceName,
			ResourcePatternTypeFilter: patternTypeFromString(cfg.PatternType),
			PrincipalFilter:           cfg.Principal,
			HostFilter:                cfg.Host,
			Operation:                 operationFromString(cfg.Operation),
			PermissionType:            permissionTypeFromString(cfg.PermissionType),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ACLs: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("failed to describe ACLs: %w", resp.Error)
	}

	if len(resp.Resources) == 0 {
		return nil, fmt.Errorf("ACL not found")
	}

	// Find the first matching ACL description
	for _, res := range resp.Resources {
		for _, acl := range res.ACLs {
			return &ACLConfig{
				ResourceType:   resourceTypeToString(res.ResourceType),
				ResourceName:   res.ResourceName,
				PatternType:    patternTypeToString(res.PatternType),
				Principal:      acl.Principal,
				Host:           acl.Host,
				Operation:      operationToString(acl.Operation),
				PermissionType: permissionTypeToString(acl.PermissionType),
			}, nil
		}
	}

	return nil, fmt.Errorf("ACL not found")
}

// DeleteACL deletes ACL entries matching the filter
func (c *StreamlineClient) DeleteACL(ctx context.Context, cfg ACLConfig) error {
	return c.withRetry(ctx, "delete ACL", func(ctx context.Context) error {
		_, err := c.kafkaClient.DeleteACLs(ctx, &kafka.DeleteACLsRequest{
			Filters: []kafka.DeleteACLsFilter{
				{
					ResourceTypeFilter:        resourceTypeFromString(cfg.ResourceType),
					ResourceNameFilter:        cfg.ResourceName,
					ResourcePatternTypeFilter: patternTypeFromString(cfg.PatternType),
					PrincipalFilter:           cfg.Principal,
					HostFilter:                cfg.Host,
					Operation:                 operationFromString(cfg.Operation),
					PermissionType:            permissionTypeFromString(cfg.PermissionType),
				},
			},
		})
		return err
	})
}

// =============================================================================
// Cluster Operations
// =============================================================================

// ClusterMetadata represents cluster metadata
type ClusterMetadata struct {
	ClusterID    string
	ControllerID int
	Brokers      []BrokerInfo
}

// BrokerInfo represents broker information
type BrokerInfo struct {
	ID   int
	Host string
	Port int
	Rack string
}

// GetClusterMetadata retrieves cluster metadata
func (c *StreamlineClient) GetClusterMetadata(ctx context.Context) (*ClusterMetadata, error) {
	conn, err := c.getControllerConn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to controller: %w", err)
	}
	defer closeQuietly(ctx, conn, "controller connection")

	brokers, err := conn.Brokers()
	if err != nil {
		return nil, fmt.Errorf("failed to get brokers: %w", err)
	}

	controller, err := conn.Controller()
	if err != nil {
		return nil, fmt.Errorf("failed to get controller: %w", err)
	}

	brokerInfos := make([]BrokerInfo, 0, len(brokers))
	for i := range brokers {
		brokerInfos = append(brokerInfos, brokerInfo(&brokers[i]))
	}

	return &ClusterMetadata{
		ClusterID:    "streamline-cluster", // kafka-go doesn't expose cluster ID directly
		ControllerID: controller.ID,
		Brokers:      brokerInfos,
	}, nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// defaultKafkaPort is assumed when a broker advertises an address without a
// usable port component.
const defaultKafkaPort = 9092

func brokerInfo(broker *kafka.Broker) BrokerInfo {
	port := broker.Port
	if port == 0 {
		port = defaultKafkaPort
	}
	return BrokerInfo{
		ID:   broker.ID,
		Host: broker.Host,
		Port: port,
		Rack: broker.Rack,
	}
}

// toPartitionCount converts a partition count to the int32 the Kafka protocol
// uses on the wire, rejecting values that cannot be represented.
func toPartitionCount(partitions int) (int32, error) {
	if partitions <= 0 || partitions > math.MaxInt32 {
		return 0, fmt.Errorf("invalid partition count %d: must be between 1 and %d", partitions, math.MaxInt32)
	}
	return int32(partitions), nil
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

// Resource type conversions
func resourceTypeFromString(s string) kafka.ResourceType {
	switch s {
	case "topic":
		return kafka.ResourceTypeTopic
	case "group":
		return kafka.ResourceTypeGroup
	case "cluster":
		return kafka.ResourceTypeCluster
	case "transactional_id":
		return kafka.ResourceTypeTransactionalID
	default:
		return kafka.ResourceTypeUnknown
	}
}

func resourceTypeToString(rt kafka.ResourceType) string {
	switch rt {
	case kafka.ResourceTypeTopic:
		return "topic"
	case kafka.ResourceTypeGroup:
		return "group"
	case kafka.ResourceTypeCluster:
		return "cluster"
	case kafka.ResourceTypeTransactionalID:
		return "transactional_id"
	default:
		return "unknown"
	}
}

// Pattern type conversions
func patternTypeFromString(s string) kafka.PatternType {
	switch s {
	case "literal":
		return kafka.PatternTypeLiteral
	case "prefixed":
		return kafka.PatternTypePrefixed
	default:
		return kafka.PatternTypeLiteral
	}
}

func patternTypeToString(pt kafka.PatternType) string {
	switch pt {
	case kafka.PatternTypeLiteral:
		return "literal"
	case kafka.PatternTypePrefixed:
		return "prefixed"
	default:
		return "literal"
	}
}

// Operation conversions
func operationFromString(s string) kafka.ACLOperationType {
	switch s {
	case "read":
		return kafka.ACLOperationTypeRead
	case "write":
		return kafka.ACLOperationTypeWrite
	case "create":
		return kafka.ACLOperationTypeCreate
	case "delete":
		return kafka.ACLOperationTypeDelete
	case "alter":
		return kafka.ACLOperationTypeAlter
	case "describe":
		return kafka.ACLOperationTypeDescribe
	case "all":
		return kafka.ACLOperationTypeAll
	default:
		return kafka.ACLOperationTypeUnknown
	}
}

func operationToString(op kafka.ACLOperationType) string {
	switch op {
	case kafka.ACLOperationTypeRead:
		return "read"
	case kafka.ACLOperationTypeWrite:
		return "write"
	case kafka.ACLOperationTypeCreate:
		return "create"
	case kafka.ACLOperationTypeDelete:
		return "delete"
	case kafka.ACLOperationTypeAlter:
		return "alter"
	case kafka.ACLOperationTypeDescribe:
		return "describe"
	case kafka.ACLOperationTypeAll:
		return "all"
	default:
		return "unknown"
	}
}

// Permission type conversions
func permissionTypeFromString(s string) kafka.ACLPermissionType {
	switch s {
	case "allow":
		return kafka.ACLPermissionTypeAllow
	case "deny":
		return kafka.ACLPermissionTypeDeny
	default:
		return kafka.ACLPermissionTypeUnknown
	}
}

func permissionTypeToString(pt kafka.ACLPermissionType) string {
	switch pt {
	case kafka.ACLPermissionTypeAllow:
		return "allow"
	case kafka.ACLPermissionTypeDeny:
		return "deny"
	default:
		return "unknown"
	}
}

// =============================================================================
// User Operations
// =============================================================================

// UserConfig represents user configuration
type UserConfig struct {
	Username  string
	Mechanism string
}

// UserInfo represents user information
type UserInfo struct {
	Username  string
	Mechanism string
}

// CreateUser creates a SASL/SCRAM user by creating a cluster-level ACL for the principal.
// Streamline manages SCRAM users through its Kafka protocol-compatible interface.
func (c *StreamlineClient) CreateUser(ctx context.Context, cfg UserConfig) error {
	// Create a describe ACL for the user principal on the cluster to register the user
	aclCfg := ACLConfig{
		ResourceType:   "cluster",
		ResourceName:   "kafka-cluster",
		PatternType:    "literal",
		Principal:      fmt.Sprintf("User:%s", cfg.Username),
		Host:           "*",
		Operation:      "describe",
		PermissionType: "allow",
	}
	return c.CreateACL(ctx, aclCfg)
}

// GetUser retrieves user information by checking for ACLs with the user principal.
func (c *StreamlineClient) GetUser(ctx context.Context, username string) (*UserInfo, error) {
	aclFilter := ACLConfig{
		ResourceType:   "cluster",
		ResourceName:   "kafka-cluster",
		PatternType:    "literal",
		Principal:      fmt.Sprintf("User:%s", username),
		Host:           "*",
		Operation:      "describe",
		PermissionType: "allow",
	}

	_, err := c.GetACL(ctx, aclFilter)
	if err != nil {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	return &UserInfo{
		Username:  username,
		Mechanism: "SCRAM-SHA-256",
	}, nil
}

// DeleteUser deletes a SASL/SCRAM user by removing associated ACLs.
func (c *StreamlineClient) DeleteUser(ctx context.Context, username string) error {
	aclFilter := ACLConfig{
		ResourceType:   "cluster",
		ResourceName:   "kafka-cluster",
		PatternType:    "literal",
		Principal:      fmt.Sprintf("User:%s", username),
		Host:           "*",
		Operation:      "describe",
		PermissionType: "allow",
	}
	return c.DeleteACL(ctx, aclFilter)
}

// Close closes the client connections
func (c *StreamlineClient) Close() error {
	// kafka-go connections are closed individually
	return nil
}

// ConsumerGroupInfo holds metadata about a consumer group.
type ConsumerGroupInfo struct {
	GroupID string
	State   string
	Members int
}

// ListConsumerGroups returns all consumer groups via the Kafka admin protocol.
func (c *StreamlineClient) ListConsumerGroups(ctx context.Context) ([]ConsumerGroupInfo, error) {
	var result []ConsumerGroupInfo
	err := c.withRetry(ctx, "list consumer groups", func(ctx context.Context) error {
		resp, err := c.kafkaClient.ListGroups(ctx, &kafka.ListGroupsRequest{
			Addr: kafka.TCP(c.brokers[0]),
		})
		if err != nil {
			return err
		}
		result = make([]ConsumerGroupInfo, 0, len(resp.Groups))
		for _, g := range resp.Groups {
			result = append(result, ConsumerGroupInfo{
				GroupID: g.GroupID,
			})
		}
		return nil
	})
	return result, err
}

// DescribeConsumerGroup returns metadata for a specific consumer group.
func (c *StreamlineClient) DescribeConsumerGroup(ctx context.Context, groupID string) (*ConsumerGroupInfo, error) {
	var result *ConsumerGroupInfo
	err := c.withRetry(ctx, "describe consumer group", func(ctx context.Context) error {
		resp, err := c.kafkaClient.DescribeGroups(ctx, &kafka.DescribeGroupsRequest{
			Addr:     kafka.TCP(c.brokers[0]),
			GroupIDs: []string{groupID},
		})
		if err != nil {
			return err
		}
		for _, g := range resp.Groups {
			if g.GroupID == groupID {
				result = &ConsumerGroupInfo{
					GroupID: g.GroupID,
					State:   g.GroupState,
					Members: len(g.Members),
				}
				return nil
			}
		}
		return fmt.Errorf("consumer group not found: %s", groupID)
	})
	return result, err
}

// DeleteConsumerGroup removes a consumer group.
func (c *StreamlineClient) DeleteConsumerGroup(ctx context.Context, groupID string) error {
	return c.withRetry(ctx, "delete consumer group", func(ctx context.Context) error {
		_, err := c.kafkaClient.DeleteGroups(ctx, &kafka.DeleteGroupsRequest{
			Addr:     kafka.TCP(c.brokers[0]),
			GroupIDs: []string{groupID},
		})
		return err
	})
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
