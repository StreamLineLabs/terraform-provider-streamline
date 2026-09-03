// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"
	"fmt"
	"net"

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
		groups, err := c.listConsumerGroupsAt(ctx, kafka.TCP(c.brokers[0]))
		if err != nil {
			return err
		}
		result = groups
		return nil
	})
	return result, err
}

// DescribeConsumerGroup returns metadata for a specific consumer group.
func (c *StreamlineClient) DescribeConsumerGroup(ctx context.Context, groupID string) (*ConsumerGroupInfo, error) {
	var result *ConsumerGroupInfo
	err := c.withRetry(ctx, "describe consumer group", func(ctx context.Context) error {
		coordinatorAddr, err := c.resolveGroupCoordinatorAddr(ctx, groupID)
		if err != nil {
			return err
		}

		resp, err := c.kafkaClient.DescribeGroups(ctx, &kafka.DescribeGroupsRequest{
			Addr:     coordinatorAddr,
			GroupIDs: []string{groupID},
		})
		if err != nil {
			return err
		}
		if resp == nil {
			return fmt.Errorf("failed to describe consumer group %q: broker returned an empty response", groupID)
		}
		if len(resp.Groups) != 1 || resp.Groups[0].GroupID != groupID {
			return fmt.Errorf(
				"failed to describe consumer group %q: expected exactly one requested-group result, got %d",
				groupID,
				len(resp.Groups),
			)
		}
		group := resp.Groups[0]
		if group.Error != nil {
			switch {
			case errors.Is(group.Error, kafka.GroupIdNotFound):
				return NewNotFoundError("consumer group", groupID, group.Error)
			case errors.Is(group.Error, kafka.GroupCoordinatorNotAvailable):
				return c.classifyCoordinatorUnavailable(ctx, "describe consumer group", groupID, group.Error)
			default:
				return fmt.Errorf("failed to describe consumer group %q: %w", groupID, group.Error)
			}
		}
		result = &ConsumerGroupInfo{
			GroupID: group.GroupID,
			State:   group.GroupState,
			Members: len(group.Members),
		}
		return nil
	})
	return result, err
}

// DeleteConsumerGroup removes a consumer group.
func (c *StreamlineClient) DeleteConsumerGroup(ctx context.Context, groupID string) error {
	return c.withRetry(ctx, "delete consumer group", func(ctx context.Context) error {
		coordinatorAddr, err := c.resolveGroupCoordinatorAddr(ctx, groupID)
		if err != nil {
			return err
		}

		resp, err := c.kafkaClient.DeleteGroups(ctx, &kafka.DeleteGroupsRequest{
			Addr:     coordinatorAddr,
			GroupIDs: []string{groupID},
		})
		if err != nil {
			return err
		}
		if resp == nil {
			return fmt.Errorf("failed to delete consumer group %q: broker returned an empty response", groupID)
		}
		if len(resp.Errors) != 1 {
			return fmt.Errorf(
				"failed to delete consumer group %q: expected exactly one requested-group result, got %d",
				groupID,
				len(resp.Errors),
			)
		}
		resultErr, ok := resp.Errors[groupID]
		if !ok {
			return fmt.Errorf("failed to delete consumer group %q: broker response omitted the requested group", groupID)
		}
		switch {
		case resultErr == nil:
			return nil
		case errors.Is(resultErr, kafka.GroupIdNotFound):
			return NewNotFoundError("consumer group", groupID, resultErr)
		case errors.Is(resultErr, kafka.GroupCoordinatorNotAvailable):
			return c.classifyCoordinatorUnavailable(ctx, "delete consumer group", groupID, resultErr)
		default:
			return fmt.Errorf("failed to delete consumer group %q: %w", groupID, resultErr)
		}
	})
}

func (c *StreamlineClient) classifyCoordinatorUnavailable(
	ctx context.Context,
	operation string,
	groupID string,
	cause error,
) error {
	return classifyCoordinatorUnavailableWith(
		ctx,
		operation,
		groupID,
		cause,
		c.consumerGroupPresentOnAnyBroker,
	)
}

func classifyCoordinatorUnavailableWith(
	ctx context.Context,
	operation string,
	groupID string,
	cause error,
	confirmPresent func(context.Context, string) (bool, error),
) error {
	present, err := confirmPresent(ctx, groupID)
	if err != nil {
		return fmt.Errorf(
			"%s %q returned CoordinatorNotAvailable and all-broker ListGroups confirmation failed: %w",
			operation,
			groupID,
			errors.Join(cause, err),
		)
	}
	if present {
		return fmt.Errorf(
			"%s %q returned transient CoordinatorNotAvailable although ListGroups still includes it: %w",
			operation,
			groupID,
			cause,
		)
	}
	return NewNotFoundError("consumer group", groupID, cause)
}

func (c *StreamlineClient) consumerGroupPresentOnAnyBroker(
	ctx context.Context,
	groupID string,
) (bool, error) {
	brokerAddrs, err := c.advertisedBrokerAddrs(ctx)
	if err != nil {
		return false, err
	}

	present := false
	var confirmationErr error
	for _, brokerAddr := range brokerAddrs {
		groups, err := c.listConsumerGroupsAt(ctx, brokerAddr)
		if err != nil {
			confirmationErr = errors.Join(confirmationErr, err)
			continue
		}
		for _, group := range groups {
			if group.GroupID == groupID {
				present = true
			}
		}
	}
	if confirmationErr != nil {
		return false, confirmationErr
	}
	return present, nil
}

func (c *StreamlineClient) listConsumerGroupsAt(
	ctx context.Context,
	brokerAddr net.Addr,
) ([]ConsumerGroupInfo, error) {
	resp, err := c.kafkaClient.ListGroups(ctx, &kafka.ListGroupsRequest{
		Addr: brokerAddr,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("broker %s returned an empty ListGroups response", brokerAddr)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("broker %s failed ListGroups: %w", brokerAddr, resp.Error)
	}

	groups := make([]ConsumerGroupInfo, 0, len(resp.Groups))
	for _, group := range resp.Groups {
		groups = append(groups, ConsumerGroupInfo{GroupID: group.GroupID})
	}
	return groups, nil
}
