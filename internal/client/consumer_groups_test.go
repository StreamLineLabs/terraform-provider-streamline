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
	"github.com/segmentio/kafka-go/protocol/deletegroups"
	"github.com/segmentio/kafka-go/protocol/describegroups"
	"github.com/segmentio/kafka-go/protocol/findcoordinator"
	"github.com/segmentio/kafka-go/protocol/listgroups"
	"github.com/segmentio/kafka-go/protocol/metadata"
)

func TestListConsumerGroupsChecksTopLevelError(t *testing.T) {
	t.Parallel()

	client := newKafkaRoundTripTestClient(func(
		_ context.Context,
		_ net.Addr,
		request kafka.Request,
	) (kafka.Response, error) {
		if _, ok := request.(*listgroups.Request); !ok {
			t.Fatalf("request type = %T, want *listgroups.Request", request)
		}
		return &listgroups.Response{
			ErrorCode: int16(kafka.GroupAuthorizationFailed),
		}, nil
	})

	_, err := client.ListConsumerGroups(context.Background())
	if !errors.Is(err, kafka.GroupAuthorizationFailed) {
		t.Fatalf("ListConsumerGroups() error = %v, want GroupAuthorizationFailed", err)
	}
}

func TestDescribeConsumerGroupRejectsEmptyResponse(t *testing.T) {
	t.Parallel()

	client := newKafkaRoundTripTestClient(func(
		_ context.Context,
		addr net.Addr,
		request kafka.Request,
	) (kafka.Response, error) {
		switch req := request.(type) {
		case *findcoordinator.Request:
			if req.Key != "workers" || req.KeyType != int8(kafka.CoordinatorKeyTypeConsumer) {
				t.Fatalf("FindCoordinator request = %+v, want consumer group workers", req)
			}
			return testFindCoordinatorResponse(2, "coordinator.example.test", 19092), nil
		case *describegroups.Request:
			if addr.String() != "coordinator.example.test:19092" {
				t.Fatalf("DescribeGroups address = %q, want coordinator.example.test:19092", addr)
			}
			return &describegroups.Response{}, nil
		default:
			t.Fatalf("unexpected request type %T", request)
			return nil, nil
		}
	})

	_, err := client.DescribeConsumerGroup(context.Background(), "workers")
	if err == nil || IsNotFound(err) || !strings.Contains(err.Error(), "exactly one requested-group result") {
		t.Fatalf("DescribeConsumerGroup() error = %v, want protocol response error", err)
	}
}

func TestCoordinatorUnavailableRequiresSuccessfulListGroupsConfirmation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		brokerTwoGroups []listgroups.ResponseGroup
		brokerTwoError  int16
		notFound        bool
	}{
		{
			name:     "absent from every broker is not found",
			notFound: true,
		},
		{
			name: "one broker omission is not authoritative",
			brokerTwoGroups: []listgroups.ResponseGroup{{
				GroupID: "workers",
			}},
		},
		{
			name:           "one broker error prevents absence confirmation",
			brokerTwoError: int16(kafka.GroupAuthorizationFailed),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			listCalls := make(map[string]int)
			client := newKafkaRoundTripTestClient(func(
				_ context.Context,
				addr net.Addr,
				request kafka.Request,
			) (kafka.Response, error) {
				switch request.(type) {
				case *findcoordinator.Request:
					return testFindCoordinatorResponse(2, "broker-two.example.test", 19092), nil
				case *describegroups.Request:
					if addr.String() != "broker-two.example.test:19092" {
						t.Fatalf("DescribeGroups address = %q, want broker-two.example.test:19092", addr)
					}
					return &describegroups.Response{
						Groups: []describegroups.ResponseGroup{{
							GroupID:   "workers",
							ErrorCode: int16(kafka.GroupCoordinatorNotAvailable),
						}},
					}, nil
				case *metadata.Request:
					return testMetadataResponse(1,
						metadata.ResponseBroker{NodeID: 1, Host: "broker-one.example.test", Port: 19091},
						metadata.ResponseBroker{NodeID: 2, Host: "broker-two.example.test", Port: 19092},
					), nil
				case *listgroups.Request:
					listCalls[addr.String()]++
					switch addr.String() {
					case "broker-one.example.test:19091":
						return &listgroups.Response{}, nil
					case "broker-two.example.test:19092":
						return &listgroups.Response{
							ErrorCode: tt.brokerTwoError,
							Groups:    tt.brokerTwoGroups,
						}, nil
					default:
						t.Fatalf("ListGroups sent to unadvertised broker %q", addr)
						return nil, nil
					}
				default:
					t.Fatalf("unexpected request type %T", request)
					return nil, nil
				}
			})

			_, err := client.DescribeConsumerGroup(context.Background(), "workers")
			if listCalls["broker-one.example.test:19091"] != 1 ||
				listCalls["broker-two.example.test:19092"] != 1 {
				t.Fatalf("ListGroups calls = %v, want every advertised broker exactly once", listCalls)
			}
			if tt.notFound {
				if !IsNotFound(err) {
					t.Fatalf("error = %v, want confirmed not found", err)
				}
				return
			}
			if err == nil || IsNotFound(err) {
				t.Fatalf("error = %v, want retriable non-not-found", err)
			}
			if tt.brokerTwoError != 0 && !errors.Is(err, kafka.GroupAuthorizationFailed) {
				t.Fatalf("error = %v, want GroupAuthorizationFailed cause", err)
			}
		})
	}
}

func TestDeleteConsumerGroupRequiresRequestedResult(t *testing.T) {
	t.Parallel()

	client := newKafkaRoundTripTestClient(func(
		_ context.Context,
		addr net.Addr,
		request kafka.Request,
	) (kafka.Response, error) {
		switch req := request.(type) {
		case *findcoordinator.Request:
			if req.Key != "workers" || req.KeyType != int8(kafka.CoordinatorKeyTypeConsumer) {
				t.Fatalf("FindCoordinator request = %+v, want consumer group workers", req)
			}
			return testFindCoordinatorResponse(2, "coordinator.example.test", 19092), nil
		case *deletegroups.Request:
			if addr.String() != "coordinator.example.test:19092" {
				t.Fatalf("DeleteGroups address = %q, want coordinator.example.test:19092", addr)
			}
			return &deletegroups.Response{}, nil
		default:
			t.Fatalf("unexpected request type %T", request)
			return nil, nil
		}
	})

	err := client.DeleteConsumerGroup(context.Background(), "workers")
	if err == nil || IsNotFound(err) || !strings.Contains(err.Error(), "exactly one requested-group result") {
		t.Fatalf("DeleteConsumerGroup() error = %v, want missing-result error", err)
	}
}
