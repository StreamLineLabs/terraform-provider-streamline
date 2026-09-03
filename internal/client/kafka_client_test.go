// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"net"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/protocol/findcoordinator"
	"github.com/segmentio/kafka-go/protocol/metadata"
)

type kafkaRoundTripFunc func(context.Context, net.Addr, kafka.Request) (kafka.Response, error)

func (f kafkaRoundTripFunc) RoundTrip(
	ctx context.Context,
	addr net.Addr,
	req kafka.Request,
) (kafka.Response, error) {
	return f(ctx, addr, req)
}

func newKafkaRoundTripTestClient(roundTrip kafkaRoundTripFunc) *StreamlineClient {
	return &StreamlineClient{
		brokers:    []string{"broker.example.test:9092"},
		maxRetries: 1,
		kafkaClient: &kafka.Client{
			Addr:      kafka.TCP("broker.example.test:9092"),
			Transport: roundTrip,
		},
	}
}

func testMetadataResponse(
	controllerID int32,
	brokers ...metadata.ResponseBroker,
) *metadata.Response {
	return &metadata.Response{
		Brokers:      brokers,
		ControllerID: controllerID,
	}
}

func testFindCoordinatorResponse(nodeID int32, host string, port int32) *findcoordinator.Response {
	return &findcoordinator.Response{
		NodeID: nodeID,
		Host:   host,
		Port:   port,
	}
}
