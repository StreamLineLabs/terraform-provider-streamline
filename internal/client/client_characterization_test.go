package client

import (
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

	if got := resourceTypeFromString("topic"); got != kafka.ResourceTypeTopic {
		t.Fatalf("resource type = %v", got)
	}
	if got := resourceTypeToString(kafka.ResourceTypeGroup); got != "group" {
		t.Fatalf("resource type string = %q", got)
	}
	if got := patternTypeFromString("prefixed"); got != kafka.PatternTypePrefixed {
		t.Fatalf("pattern type = %v", got)
	}
	if got := patternTypeToString(kafka.PatternTypeLiteral); got != "literal" {
		t.Fatalf("pattern type string = %q", got)
	}
	if got := operationFromString("write"); got != kafka.ACLOperationTypeWrite {
		t.Fatalf("operation = %v", got)
	}
	if got := operationToString(kafka.ACLOperationTypeRead); got != "read" {
		t.Fatalf("operation string = %q", got)
	}
	if got := permissionTypeFromString("deny"); got != kafka.ACLPermissionTypeDeny {
		t.Fatalf("permission = %v", got)
	}
	if got := permissionTypeToString(kafka.ACLPermissionTypeAllow); got != "allow" {
		t.Fatalf("permission string = %q", got)
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
