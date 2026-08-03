// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

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
