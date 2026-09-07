package client

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

func TestNewStreamlineClientAppliesDefaults(t *testing.T) {
	t.Parallel()

	got, err := NewStreamlineClient(Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("NewStreamlineClient() error = %v", err)
	}

	if got.timeout != 30*time.Second {
		t.Fatalf("timeout = %v, want 30s", got.timeout)
	}
	if got.maxRetries != 3 {
		t.Fatalf("maxRetries = %d, want 3", got.maxRetries)
	}
	if got.kafkaClient == nil || got.dialer == nil {
		t.Fatal("expected Kafka client and dialer to be initialized")
	}
}

func TestNewStreamlineClientRejectsUnsupportedSASL(t *testing.T) {
	t.Parallel()

	_, err := NewStreamlineClient(Config{
		Brokers:       []string{"localhost:9092"},
		SASLMechanism: "unsupported",
	})
	if err == nil {
		t.Fatal("expected unsupported SASL mechanism error")
	}
}

func TestNewStreamlineClientSeparatesConnectionAndRequestTimeouts(t *testing.T) {
	t.Parallel()

	got, err := NewStreamlineClient(Config{
		Brokers:           []string{"localhost:9092"},
		ConnectionTimeout: 5 * time.Second,
		Timeout:           2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewStreamlineClient() error = %v", err)
	}

	if got.dialer.Timeout != 5*time.Second {
		t.Fatalf("dialer timeout = %v, want 5s", got.dialer.Timeout)
	}
	if got.kafkaClient.Timeout != 2*time.Second {
		t.Fatalf("request timeout = %v, want 2s", got.kafkaClient.Timeout)
	}
	if got.httpClient.Timeout != 2*time.Second {
		t.Fatalf("HTTP timeout = %v, want 2s", got.httpClient.Timeout)
	}
}

func TestACLConversions(t *testing.T) {
	t.Parallel()

	resourceTypes := map[string]kafka.ResourceType{
		"topic":            kafka.ResourceTypeTopic,
		"group":            kafka.ResourceTypeGroup,
		"cluster":          kafka.ResourceTypeCluster,
		"transactional_id": kafka.ResourceTypeTransactionalID,
		"delegation_token": kafka.ResourceTypeDelegationToken,
	}
	for value, kafkaValue := range resourceTypes {
		got, err := resourceTypeFromString(value)
		if err != nil || got != kafkaValue {
			t.Fatalf("resourceTypeFromString(%q) = %v, %v", value, got, err)
		}
		roundTrip, err := resourceTypeToString(kafkaValue)
		if err != nil || roundTrip != value {
			t.Fatalf("resourceTypeToString(%v) = %q, %v", kafkaValue, roundTrip, err)
		}
	}

	creationPatternTypes := map[string]kafka.PatternType{
		"literal":  kafka.PatternTypePrefixed,
		"prefixed": kafka.PatternTypeLiteral,
	}
	for value, kafkaValue := range creationPatternTypes {
		got, err := creationPatternTypeFromString(value)
		if err != nil || got != kafkaValue {
			t.Fatalf("creationPatternTypeFromString(%q) = %v, %v", value, got, err)
		}
	}
	filterPatternTypes := map[string]kafka.PatternType{
		"literal":  kafka.PatternTypeMatch,
		"prefixed": kafka.PatternTypeLiteral,
	}
	for value, kafkaValue := range filterPatternTypes {
		got, err := filterPatternTypeFromString(value)
		if err != nil || got != kafkaValue {
			t.Fatalf("filterPatternTypeFromString(%q) = %v, %v", value, got, err)
		}
	}
	responsePatternTypes := map[kafka.PatternType]string{
		kafka.PatternTypeLiteral:  "literal",
		kafka.PatternTypePrefixed: "prefixed",
	}
	for kafkaValue, value := range responsePatternTypes {
		got, err := patternTypeToString(kafkaValue)
		if err != nil || got != value {
			t.Fatalf("patternTypeToString(%v) = %q, %v", kafkaValue, got, err)
		}
	}

	operations := map[string]kafka.ACLOperationType{
		"read":             kafka.ACLOperationTypeRead,
		"write":            kafka.ACLOperationTypeWrite,
		"create":           kafka.ACLOperationTypeCreate,
		"delete":           kafka.ACLOperationTypeDelete,
		"alter":            kafka.ACLOperationTypeAlter,
		"describe":         kafka.ACLOperationTypeDescribe,
		"cluster_action":   kafka.ACLOperationTypeClusterAction,
		"describe_configs": kafka.ACLOperationTypeDescribeConfigs,
		"alter_configs":    kafka.ACLOperationTypeAlterConfigs,
		"idempotent_write": kafka.ACLOperationTypeIdempotentWrite,
	}
	for value, kafkaValue := range operations {
		got, err := operationFromString(value)
		if err != nil || got != kafkaValue {
			t.Fatalf("operationFromString(%q) = %v, %v", value, got, err)
		}
		roundTrip, err := operationToString(kafkaValue)
		if err != nil || roundTrip != value {
			t.Fatalf("operationToString(%v) = %q, %v", kafkaValue, roundTrip, err)
		}
	}

	permissionTypes := map[string]kafka.ACLPermissionType{
		"allow": kafka.ACLPermissionTypeAllow,
		"deny":  kafka.ACLPermissionTypeDeny,
	}
	for value, kafkaValue := range permissionTypes {
		got, err := permissionTypeFromString(value)
		if err != nil || got != kafkaValue {
			t.Fatalf("permissionTypeFromString(%q) = %v, %v", value, got, err)
		}
		roundTrip, err := permissionTypeToString(kafkaValue)
		if err != nil || roundTrip != value {
			t.Fatalf("permissionTypeToString(%v) = %q, %v", kafkaValue, roundTrip, err)
		}
	}

	if _, err := resourceTypeFromString("unsupported"); err == nil {
		t.Fatal("expected unsupported resource type to fail")
	}
	if _, err := creationPatternTypeFromString("match"); err == nil {
		t.Fatal("expected filter-only match pattern to fail")
	}
	if _, err := filterPatternTypeFromString("match"); err == nil {
		t.Fatal("expected unsupported filter pattern to fail")
	}
	if got, err := operationFromString("all"); err != nil || got != kafka.ACLOperationTypeAll {
		t.Fatalf("legacy all operation must remain readable, got %v, %v", got, err)
	}
	if _, err := operationFromString("unsupported"); err == nil {
		t.Fatal("expected unsupported operation to fail")
	}
	if _, err := permissionTypeFromString("unsupported"); err == nil {
		t.Fatal("expected unsupported permission type to fail")
	}
}

func TestValidateManagedACLConfigRejectsBroadDeleteFilters(t *testing.T) {
	t.Parallel()

	base := ACLConfig{
		ResourceType:   "topic",
		ResourceName:   "events",
		PatternType:    "literal",
		Principal:      "User:alice",
		Host:           "10.0.0.10",
		Operation:      "read",
		PermissionType: "allow",
	}

	tests := map[string]ACLConfig{
		"resource wildcard": func() ACLConfig {
			cfg := base
			cfg.ResourceName = "*"
			return cfg
		}(),
		"principal wildcard": func() ACLConfig {
			cfg := base
			cfg.Principal = "User:*"
			return cfg
		}(),
		"host wildcard": func() ACLConfig {
			cfg := base
			cfg.Host = "*"
			return cfg
		}(),
		"all operation": func() ACLConfig {
			cfg := base
			cfg.Operation = "all"
			return cfg
		}(),
		"delegation token": func() ACLConfig {
			cfg := base
			cfg.ResourceType = "delegation_token"
			return cfg
		}(),
		"match pattern": func() ACLConfig {
			cfg := base
			cfg.PatternType = "match"
			return cfg
		}(),
	}
	for name, cfg := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateManagedACLConfig(cfg); err == nil {
				t.Fatal("expected unsafe managed ACL to be rejected")
			}
		})
	}
}

func TestValidateCreateACLResultsRejectsMissingBrokerResult(t *testing.T) {
	t.Parallel()

	if err := validateCreateACLResults(nil); err == nil {
		t.Fatal("expected an empty lite-broker response to fail")
	}
	if err := validateCreateACLResults([]error{nil}); err != nil {
		t.Fatalf("expected one successful result, got %v", err)
	}
}

func TestValidateDeleteACLResultsRejectsEntryError(t *testing.T) {
	t.Parallel()

	cfg := ACLConfig{
		ResourceType:   "topic",
		ResourceName:   "events",
		PatternType:    "literal",
		Principal:      "User:alice",
		Host:           "*",
		Operation:      "read",
		PermissionType: "allow",
	}
	entryErr := errors.New("authorization failed")
	err := validateDeleteACLResults(cfg, []kafka.DeleteACLsResult{{
		MatchingACLs: []kafka.DeleteACLsMatchingACLs{{
			Error: entryErr,
		}},
	}})
	if !errors.Is(err, entryErr) {
		t.Fatalf("expected entry error to be preserved, got %v", err)
	}
}

func TestValidateDeleteACLResultsRejectsBroadMatch(t *testing.T) {
	t.Parallel()

	cfg := ACLConfig{
		ResourceType:   "topic",
		ResourceName:   "events",
		PatternType:    "literal",
		Principal:      "User:alice",
		Host:           "*",
		Operation:      "read",
		PermissionType: "allow",
	}
	err := validateDeleteACLResults(cfg, []kafka.DeleteACLsResult{{
		MatchingACLs: []kafka.DeleteACLsMatchingACLs{{
			ResourceType:        kafka.ResourceTypeTopic,
			ResourceName:        "events",
			ResourcePatternType: kafka.PatternTypeLiteral,
			Principal:           "User:alice",
			Host:                "*",
			Operation:           kafka.ACLOperationTypeWrite,
			PermissionType:      kafka.ACLPermissionTypeAllow,
		}},
	}})
	if err == nil {
		t.Fatal("expected an unexpectedly broad match to fail")
	}
}

func TestUpdateTopicRejectsConfigurationChanges(t *testing.T) {
	t.Parallel()

	c, err := NewStreamlineClient(Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("NewStreamlineClient() error = %v", err)
	}

	err = c.UpdateTopic(context.Background(), TopicConfig{
		Name:       "events",
		Partitions: 3,
		Config:     map[string]string{"retention.ms": "1000"},
	})
	if err == nil {
		t.Fatal("expected topic configuration update to be rejected")
	}
}

func TestToPartitionCount(t *testing.T) {
	t.Parallel()

	got, err := toPartitionCount(math.MaxInt32)
	if err != nil || got != math.MaxInt32 {
		t.Fatalf("toPartitionCount(max) = %d, %v", got, err)
	}
	for _, value := range []int{0, -1, math.MaxInt32 + 1} {
		if _, err := toPartitionCount(value); err == nil {
			t.Fatalf("toPartitionCount(%d) expected error", value)
		}
	}
}
