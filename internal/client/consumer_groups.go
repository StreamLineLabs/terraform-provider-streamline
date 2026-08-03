// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

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
