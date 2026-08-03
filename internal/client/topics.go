// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"math"

	"github.com/segmentio/kafka-go"
)

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

// toPartitionCount converts a partition count to the int32 the Kafka protocol
// uses on the wire, rejecting values that cannot be represented.
func toPartitionCount(partitions int) (int32, error) {
	if partitions <= 0 || partitions > math.MaxInt32 {
		return 0, fmt.Errorf("invalid partition count %d: must be between 1 and %d", partitions, math.MaxInt32)
	}
	return int32(partitions), nil
}
