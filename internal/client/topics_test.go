// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/protocol/createtopics"
	"github.com/segmentio/kafka-go/protocol/deletetopics"
	"github.com/segmentio/kafka-go/protocol/metadata"
)

func TestGetTopicUsesMetadataWithoutAutoCreation(t *testing.T) {
	t.Parallel()

	sawMetadataRequest := false
	client := newKafkaRoundTripTestClient(func(
		_ context.Context,
		_ net.Addr,
		request kafka.Request,
	) (kafka.Response, error) {
		req, ok := request.(*metadata.Request)
		if !ok {
			t.Fatalf("request type = %T, want *metadata.Request", request)
		}
		sawMetadataRequest = true
		if req.AllowAutoTopicCreation {
			t.Fatal("metadata lookup must disable automatic topic creation")
		}
		if len(req.TopicNames) != 1 || req.TopicNames[0] != "events" {
			t.Fatalf("metadata topics = %v, want [events]", req.TopicNames)
		}
		return &metadata.Response{
			Brokers: []metadata.ResponseBroker{{
				NodeID: 1,
				Host:   "broker.example.test",
				Port:   9092,
			}},
			ControllerID: 1,
			Topics: []metadata.ResponseTopic{{
				Name: "events",
				Partitions: []metadata.ResponsePartition{{
					PartitionIndex: 0,
					LeaderID:       1,
					ReplicaNodes:   []int32{1},
					IsrNodes:       []int32{1},
				}},
			}},
		}, nil
	})

	got, err := client.GetTopic(context.Background(), "events")
	if err != nil {
		t.Fatalf("GetTopic() error = %v", err)
	}
	if !sawMetadataRequest {
		t.Fatal("GetTopic() did not issue a Metadata request")
	}
	if got == nil || got.Name != "events" || got.Partitions != 1 || got.ReplicationFactor != 1 {
		t.Fatalf("GetTopic() = %+v", got)
	}
}

func TestCreateTopicRequiresExactlyOneRequestedResult(t *testing.T) {
	t.Parallel()

	client := newKafkaRoundTripTestClient(func(
		_ context.Context,
		addr net.Addr,
		request kafka.Request,
	) (kafka.Response, error) {
		switch req := request.(type) {
		case *metadata.Request:
			return testMetadataResponse(2,
				metadata.ResponseBroker{NodeID: 1, Host: "broker.example.test", Port: 9092},
				metadata.ResponseBroker{NodeID: 2, Host: "controller.example.test", Port: 19092},
			), nil
		case *createtopics.Request:
			if addr.String() != "controller.example.test:19092" {
				t.Fatalf("CreateTopics address = %q, want controller.example.test:19092", addr)
			}
			if len(req.Topics) != 1 || req.Topics[0].Name != "events" {
				t.Fatalf("create topics = %+v, want exactly events", req.Topics)
			}
			return &createtopics.Response{}, nil
		default:
			t.Fatalf("request type = %T, want Metadata or CreateTopics", request)
			return nil, nil
		}
	})

	err := client.CreateTopic(context.Background(), TopicConfig{
		Name:              "events",
		Partitions:        1,
		ReplicationFactor: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "expected exactly one requested-topic result") {
		t.Fatalf("CreateTopic() error = %v, want missing-result error", err)
	}
}

func TestCreateTopicSurfacesAuthorizationResult(t *testing.T) {
	t.Parallel()

	client := newKafkaRoundTripTestClient(func(
		_ context.Context,
		addr net.Addr,
		request kafka.Request,
	) (kafka.Response, error) {
		switch request.(type) {
		case *metadata.Request:
			return testMetadataResponse(2,
				metadata.ResponseBroker{NodeID: 1, Host: "broker.example.test", Port: 9092},
				metadata.ResponseBroker{NodeID: 2, Host: "controller.example.test", Port: 19092},
			), nil
		case *createtopics.Request:
			if addr.String() != "controller.example.test:19092" {
				t.Fatalf("CreateTopics address = %q, want controller.example.test:19092", addr)
			}
			return &createtopics.Response{
				Topics: []createtopics.ResponseTopic{{
					Name:      "events",
					ErrorCode: int16(kafka.TopicAuthorizationFailed),
				}},
			}, nil
		default:
			t.Fatalf("request type = %T, want Metadata or CreateTopics", request)
			return nil, nil
		}
	})

	err := client.CreateTopic(context.Background(), TopicConfig{
		Name:              "events",
		Partitions:        1,
		ReplicationFactor: 1,
	})
	if !errors.Is(err, kafka.TopicAuthorizationFailed) {
		t.Fatalf("CreateTopic() error = %v, want TopicAuthorizationFailed", err)
	}
}

func TestCreateTopicResolvesCurrentControllerOnEveryRetry(t *testing.T) {
	t.Parallel()

	metadataCalls := 0
	createCalls := 0
	client := newKafkaRoundTripTestClient(func(
		_ context.Context,
		addr net.Addr,
		request kafka.Request,
	) (kafka.Response, error) {
		switch request.(type) {
		case *metadata.Request:
			metadataCalls++
			controllerID := int32(metadataCalls)
			return testMetadataResponse(controllerID,
				metadata.ResponseBroker{NodeID: 1, Host: "controller-one.example.test", Port: 19091},
				metadata.ResponseBroker{NodeID: 2, Host: "controller-two.example.test", Port: 19092},
			), nil
		case *createtopics.Request:
			createCalls++
			wantAddr := "controller-one.example.test:19091"
			errorCode := int16(kafka.NotController)
			if createCalls == 2 {
				wantAddr = "controller-two.example.test:19092"
				errorCode = 0
			}
			if addr.String() != wantAddr {
				t.Fatalf("CreateTopics attempt %d address = %q, want %q", createCalls, addr, wantAddr)
			}
			return &createtopics.Response{
				Topics: []createtopics.ResponseTopic{{
					Name:      "events",
					ErrorCode: errorCode,
				}},
			}, nil
		default:
			t.Fatalf("unexpected request type %T", request)
			return nil, nil
		}
	})
	client.maxRetries = 2

	err := client.CreateTopic(context.Background(), TopicConfig{
		Name:              "events",
		Partitions:        1,
		ReplicationFactor: 1,
	})
	if err != nil {
		t.Fatalf("CreateTopic() error = %v", err)
	}
	if metadataCalls != 2 || createCalls != 2 {
		t.Fatalf("calls = metadata:%d create:%d, want 2 each", metadataCalls, createCalls)
	}
}

func TestDeleteTopicResolvesCurrentControllerOnEveryRetry(t *testing.T) {
	t.Parallel()

	metadataCalls := 0
	deleteCalls := 0
	client := newKafkaRoundTripTestClient(func(
		_ context.Context,
		addr net.Addr,
		request kafka.Request,
	) (kafka.Response, error) {
		switch request.(type) {
		case *metadata.Request:
			metadataCalls++
			controllerID := int32(metadataCalls)
			return testMetadataResponse(controllerID,
				metadata.ResponseBroker{NodeID: 1, Host: "controller-one.example.test", Port: 19091},
				metadata.ResponseBroker{NodeID: 2, Host: "controller-two.example.test", Port: 19092},
			), nil
		case *deletetopics.Request:
			deleteCalls++
			wantAddr := "controller-one.example.test:19091"
			errorCode := int16(kafka.NotController)
			if deleteCalls == 2 {
				wantAddr = "controller-two.example.test:19092"
				errorCode = 0
			}
			if addr.String() != wantAddr {
				t.Fatalf("DeleteTopics attempt %d address = %q, want %q", deleteCalls, addr, wantAddr)
			}
			return &deletetopics.Response{
				Responses: []deletetopics.ResponseTopic{{
					Name:      "events",
					ErrorCode: errorCode,
				}},
			}, nil
		default:
			t.Fatalf("unexpected request type %T", request)
			return nil, nil
		}
	})
	client.maxRetries = 2

	if err := client.DeleteTopic(context.Background(), "events"); err != nil {
		t.Fatalf("DeleteTopic() error = %v", err)
	}
	if metadataCalls != 2 || deleteCalls != 2 {
		t.Fatalf("calls = metadata:%d delete:%d, want 2 each", metadataCalls, deleteCalls)
	}
}

func TestUnknownTopicCannotBeConfirmedByBrokerlessMetadata(t *testing.T) {
	t.Parallel()

	metadataCalls := 0
	client := newKafkaRoundTripTestClient(func(
		_ context.Context,
		_ net.Addr,
		request kafka.Request,
	) (kafka.Response, error) {
		if _, ok := request.(*metadata.Request); !ok {
			t.Fatalf("request type = %T, want *metadata.Request", request)
		}
		metadataCalls++
		if metadataCalls == 1 {
			return &metadata.Response{
				Brokers: []metadata.ResponseBroker{{
					NodeID: 1,
					Host:   "broker.example.test",
					Port:   9092,
				}},
				ControllerID: 1,
				Topics: []metadata.ResponseTopic{{
					Name:      "events",
					ErrorCode: int16(kafka.UnknownTopicOrPartition),
				}},
			}, nil
		}
		return &metadata.Response{}, nil
	})

	_, err := client.GetTopic(context.Background(), "events")
	if err == nil {
		t.Fatal("GetTopic() succeeded with structurally invalid confirmation metadata")
	}
	if IsNotFound(err) {
		t.Fatalf("brokerless metadata incorrectly confirmed absence: %v", err)
	}
	if !errors.Is(err, kafka.UnknownTopicOrPartition) {
		t.Fatalf("GetTopic() error = %v, want original unknown-topic cause", err)
	}
	if !strings.Contains(err.Error(), "advertised no brokers") {
		t.Fatalf("GetTopic() error = %v, want invalid metadata detail", err)
	}
}

func TestRequireSingleNamedResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		results map[string]error
		wantErr string
	}{
		{name: "success", results: map[string]error{"events": nil}},
		{name: "empty", results: nil, wantErr: "exactly one"},
		{
			name:    "extra result",
			results: map[string]error{"events": nil, "other": nil},
			wantErr: "exactly one",
		},
		{name: "wrong result", results: map[string]error{"other": nil}, wantErr: "omitted"},
		{
			name:    "authorization",
			results: map[string]error{"events": kafka.TopicAuthorizationFailed},
			wantErr: "authorization",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := requireSingleNamedResult("create topic", "events", tt.results)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestClassifyUnknownTopicRequiresIndependentConfirmedAbsence(t *testing.T) {
	t.Parallel()

	unknown := kafka.UnknownTopicOrPartition

	t.Run("confirmed absent is not found", func(t *testing.T) {
		t.Parallel()

		err := classifyUnknownTopicWith(
			context.Background(),
			"get topic",
			"events",
			unknown,
			func(context.Context) ([]TopicMetadata, error) {
				return []TopicMetadata{{Name: "other"}}, nil
			},
		)
		if !IsNotFound(err) {
			t.Fatalf("confirmed absence error = %v, want typed not found", err)
		}
	})

	t.Run("confirmed present remains retriable", func(t *testing.T) {
		t.Parallel()

		err := classifyUnknownTopicWith(
			context.Background(),
			"get topic",
			"events",
			unknown,
			func(context.Context) ([]TopicMetadata, error) {
				return []TopicMetadata{{Name: "events"}}, nil
			},
		)
		if IsNotFound(err) {
			t.Fatalf("transient metadata error was incorrectly classified as not found: %v", err)
		}
		if !errors.Is(err, unknown) {
			t.Fatalf("transient metadata error does not retain the Kafka cause: %v", err)
		}
	})

	t.Run("failed confirmation remains retriable", func(t *testing.T) {
		t.Parallel()

		confirmationErr := errors.New("metadata unavailable")
		err := classifyUnknownTopicWith(
			context.Background(),
			"delete topic",
			"events",
			unknown,
			func(context.Context) ([]TopicMetadata, error) {
				return nil, confirmationErr
			},
		)
		if IsNotFound(err) {
			t.Fatalf("failed confirmation was incorrectly classified as not found: %v", err)
		}
		if !errors.Is(err, unknown) || !errors.Is(err, confirmationErr) {
			t.Fatalf("failed confirmation did not retain both causes: %v", err)
		}
	})
}

func TestTransientUnknownTopicClassificationIsRetried(t *testing.T) {
	t.Parallel()

	c := &StreamlineClient{maxRetries: 2}
	attempts := 0
	err := c.withRetry(context.Background(), "get topic", func(ctx context.Context) error {
		attempts++
		if attempts == 2 {
			return nil
		}
		return classifyUnknownTopicWith(
			ctx,
			"get topic",
			"events",
			kafka.UnknownTopicOrPartition,
			func(context.Context) ([]TopicMetadata, error) {
				return []TopicMetadata{{Name: "events"}}, nil
			},
		)
	})
	if err != nil {
		t.Fatalf("transient unknown-topic response was not retried successfully: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestConfirmedUnknownTopicStopsRetrying(t *testing.T) {
	t.Parallel()

	c := &StreamlineClient{maxRetries: 3}
	attempts := 0
	err := c.withRetry(context.Background(), "get topic", func(ctx context.Context) error {
		attempts++
		return classifyUnknownTopicWith(
			ctx,
			"get topic",
			"events",
			kafka.UnknownTopicOrPartition,
			func(context.Context) ([]TopicMetadata, error) {
				return nil, nil
			},
		)
	})
	if !IsNotFound(err) {
		t.Fatalf("confirmed absence error = %v, want typed not found", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
