// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"
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
}

// CreateTopic creates a new topic
func (c *StreamlineClient) CreateTopic(ctx context.Context, cfg TopicConfig) error {
	return c.withRetry(ctx, "create topic", func(ctx context.Context) error {
		controllerAddr, err := c.resolveControllerAddr(ctx)
		if err != nil {
			return err
		}

		topicConfig := kafka.TopicConfig{
			Topic:             cfg.Name,
			NumPartitions:     cfg.Partitions,
			ReplicationFactor: cfg.ReplicationFactor,
		}

		if len(cfg.Config) > 0 {
			configs := make([]kafka.ConfigEntry, 0, len(cfg.Config))
			for k, v := range cfg.Config {
				configs = append(configs, kafka.ConfigEntry{
					ConfigName:  k,
					ConfigValue: v,
				})
			}
			topicConfig.ConfigEntries = configs
		}

		resp, err := c.kafkaClient.CreateTopics(ctx, &kafka.CreateTopicsRequest{
			Addr:   controllerAddr,
			Topics: []kafka.TopicConfig{topicConfig},
		})
		if err != nil {
			return fmt.Errorf("failed to create topic %q: %w", cfg.Name, err)
		}
		if resp == nil {
			return fmt.Errorf("failed to create topic %q: broker returned an empty response", cfg.Name)
		}
		return requireSingleNamedResult("create topic", cfg.Name, resp.Errors)
	})
}

// GetTopic retrieves topic metadata
func (c *StreamlineClient) GetTopic(ctx context.Context, name string) (*TopicMetadata, error) {
	var result *TopicMetadata
	err := c.withRetry(ctx, "get topic", func(ctx context.Context) error {
		metadata, err := c.readTopicMetadata(ctx, name)
		if err == nil {
			result = metadata
			return nil
		}
		if errors.Is(err, kafka.UnknownTopicOrPartition) {
			return c.classifyUnknownTopic(ctx, "get topic", name, err)
		}
		return err
	})
	return result, err
}

func (c *StreamlineClient) readTopicMetadata(ctx context.Context, name string) (*TopicMetadata, error) {
	resp, err := c.readClusterMetadata(ctx, []string{name})
	if err != nil {
		return nil, fmt.Errorf("failed to read topic metadata: %w", err)
	}
	if len(resp.Topics) != 1 || resp.Topics[0].Name != name {
		return nil, fmt.Errorf(
			"failed to read topic metadata for %q: expected exactly one requested-topic result, got %d",
			name,
			len(resp.Topics),
		)
	}
	topic := resp.Topics[0]
	if topic.Error != nil {
		return nil, fmt.Errorf("failed to read topic metadata for %q: %w", name, topic.Error)
	}
	if len(topic.Partitions) == 0 {
		return nil, fmt.Errorf("topic metadata contained no partitions: %w", kafka.UnknownTopicOrPartition)
	}
	for i := range topic.Partitions {
		partition := &topic.Partitions[i]
		if partition.Error != nil {
			return nil, fmt.Errorf(
				"failed to read topic metadata for %q partition %d: %w",
				name,
				partition.ID,
				partition.Error,
			)
		}
	}

	replicationFactor := len(topic.Partitions[0].Replicas)

	return &TopicMetadata{
		Name:              name,
		Partitions:        len(topic.Partitions),
		ReplicationFactor: replicationFactor,
	}, nil
}

// classifyUnknownTopic confirms an UnknownTopicOrPartition response against a
// separate all-topic metadata request. Kafka can return this error transiently
// while metadata converges, so it is not a confirmed absence by itself.
func (c *StreamlineClient) classifyUnknownTopic(
	ctx context.Context,
	operation string,
	name string,
	cause error,
) error {
	return classifyUnknownTopicWith(ctx, operation, name, cause, c.ListTopics)
}

func classifyUnknownTopicWith(
	ctx context.Context,
	operation string,
	name string,
	cause error,
	listTopics func(context.Context) ([]TopicMetadata, error),
) error {
	topics, err := listTopics(ctx)
	if err != nil {
		return fmt.Errorf(
			"%s for topic %q returned an unknown-topic response and independent metadata confirmation failed: %w",
			operation,
			name,
			errors.Join(cause, err),
		)
	}
	for _, topic := range topics {
		if topic.Name == name {
			return fmt.Errorf(
				"%s for topic %q returned a transient unknown-topic response although independent metadata still lists it: %w",
				operation,
				name,
				cause,
			)
		}
	}
	return NewNotFoundError("topic", name, cause)
}

// UpdateTopic increases a topic's partition count. Topic configuration changes
// are deliberately rejected because the Kafka API used by this provider cannot
// read them back or apply them reliably.
func (c *StreamlineClient) UpdateTopic(ctx context.Context, cfg TopicConfig) error {
	if len(cfg.Config) != 0 {
		return fmt.Errorf("updating topic configuration is unsupported; replace the topic to change configuration")
	}

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
		controllerAddr, err := c.resolveControllerAddr(ctx)
		if err != nil {
			return err
		}

		resp, err := c.kafkaClient.DeleteTopics(ctx, &kafka.DeleteTopicsRequest{
			Addr:   controllerAddr,
			Topics: []string{name},
		})
		if err != nil {
			return fmt.Errorf("failed to delete topic %q: %w", name, err)
		}
		if resp == nil {
			return fmt.Errorf("failed to delete topic %q: broker returned an empty response", name)
		}
		resultErr := requireSingleNamedResult("delete topic", name, resp.Errors)
		if errors.Is(resultErr, kafka.UnknownTopicOrPartition) {
			return c.classifyUnknownTopic(ctx, "delete topic", name, resultErr)
		}
		return resultErr
	})
}

// ListTopics lists all topics
func (c *StreamlineClient) ListTopics(ctx context.Context) ([]TopicMetadata, error) {
	// A nil topic list requests all visible topics. kafka.Client.Metadata sends
	// AllowAutoTopicCreation=false, so metadata reads cannot create a missing
	// topic as a side effect.
	resp, err := c.readClusterMetadata(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list topic metadata: %w", err)
	}

	topics := make([]TopicMetadata, 0, len(resp.Topics))
	for _, topic := range resp.Topics {
		if topic.Error != nil {
			return nil, fmt.Errorf("failed to list topic metadata for %q: %w", topic.Name, topic.Error)
		}
		replicationFactor := 0
		for i := range topic.Partitions {
			partition := &topic.Partitions[i]
			if partition.Error != nil {
				return nil, fmt.Errorf(
					"failed to list topic metadata for %q partition %d: %w",
					topic.Name,
					partition.ID,
					partition.Error,
				)
			}
			if replicationFactor == 0 {
				replicationFactor = len(partition.Replicas)
			}
		}
		topics = append(topics, TopicMetadata{
			Name:              topic.Name,
			Partitions:        len(topic.Partitions),
			ReplicationFactor: replicationFactor,
		})
	}

	return topics, nil
}

func requireSingleNamedResult(operation, name string, results map[string]error) error {
	if len(results) != 1 {
		return fmt.Errorf(
			"%s %q: expected exactly one requested-topic result, got %d",
			operation,
			name,
			len(results),
		)
	}
	resultErr, ok := results[name]
	if !ok {
		return fmt.Errorf("%s %q: broker response omitted the requested topic", operation, name)
	}
	if resultErr != nil {
		return fmt.Errorf("%s %q: %w", operation, name, resultErr)
	}
	return nil
}

// toPartitionCount converts a partition count to the int32 the Kafka protocol
// uses on the wire, rejecting values that cannot be represented.
func toPartitionCount(partitions int) (int32, error) {
	if partitions <= 0 || partitions > math.MaxInt32 {
		return 0, fmt.Errorf("invalid partition count %d: must be between 1 and %d", partitions, math.MaxInt32)
	}
	return int32(partitions), nil
}
