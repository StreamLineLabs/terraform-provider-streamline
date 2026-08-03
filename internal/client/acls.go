// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

// ACLConfig represents ACL configuration
type ACLConfig struct {
	ResourceType   string
	ResourceName   string
	PatternType    string
	Principal      string
	Host           string
	Operation      string
	PermissionType string
}

// CreateACL creates a new ACL
func (c *StreamlineClient) CreateACL(ctx context.Context, cfg ACLConfig) error {
	return c.withRetry(ctx, "create ACL", func(ctx context.Context) error {
		_, err := c.kafkaClient.CreateACLs(ctx, &kafka.CreateACLsRequest{
			ACLs: []kafka.ACLEntry{
				{
					ResourceType:        resourceTypeFromString(cfg.ResourceType),
					ResourceName:        cfg.ResourceName,
					ResourcePatternType: patternTypeFromString(cfg.PatternType),
					Principal:           cfg.Principal,
					Host:                cfg.Host,
					Operation:           operationFromString(cfg.Operation),
					PermissionType:      permissionTypeFromString(cfg.PermissionType),
				},
			},
		})
		return err
	})
}

// GetACL retrieves a single ACL entry matching the filter
func (c *StreamlineClient) GetACL(ctx context.Context, cfg ACLConfig) (*ACLConfig, error) {
	resp, err := c.kafkaClient.DescribeACLs(ctx, &kafka.DescribeACLsRequest{
		Filter: kafka.ACLFilter{
			ResourceTypeFilter:        resourceTypeFromString(cfg.ResourceType),
			ResourceNameFilter:        cfg.ResourceName,
			ResourcePatternTypeFilter: patternTypeFromString(cfg.PatternType),
			PrincipalFilter:           cfg.Principal,
			HostFilter:                cfg.Host,
			Operation:                 operationFromString(cfg.Operation),
			PermissionType:            permissionTypeFromString(cfg.PermissionType),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ACLs: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("failed to describe ACLs: %w", resp.Error)
	}

	if len(resp.Resources) == 0 {
		return nil, fmt.Errorf("ACL not found")
	}

	// Find the first matching ACL description
	for _, res := range resp.Resources {
		for _, acl := range res.ACLs {
			return &ACLConfig{
				ResourceType:   resourceTypeToString(res.ResourceType),
				ResourceName:   res.ResourceName,
				PatternType:    patternTypeToString(res.PatternType),
				Principal:      acl.Principal,
				Host:           acl.Host,
				Operation:      operationToString(acl.Operation),
				PermissionType: permissionTypeToString(acl.PermissionType),
			}, nil
		}
	}

	return nil, fmt.Errorf("ACL not found")
}

// DeleteACL deletes ACL entries matching the filter
func (c *StreamlineClient) DeleteACL(ctx context.Context, cfg ACLConfig) error {
	return c.withRetry(ctx, "delete ACL", func(ctx context.Context) error {
		_, err := c.kafkaClient.DeleteACLs(ctx, &kafka.DeleteACLsRequest{
			Filters: []kafka.DeleteACLsFilter{
				{
					ResourceTypeFilter:        resourceTypeFromString(cfg.ResourceType),
					ResourceNameFilter:        cfg.ResourceName,
					ResourcePatternTypeFilter: patternTypeFromString(cfg.PatternType),
					PrincipalFilter:           cfg.Principal,
					HostFilter:                cfg.Host,
					Operation:                 operationFromString(cfg.Operation),
					PermissionType:            permissionTypeFromString(cfg.PermissionType),
				},
			},
		})
		return err
	})
}

// Resource type conversions
func resourceTypeFromString(s string) kafka.ResourceType {
	switch s {
	case "topic":
		return kafka.ResourceTypeTopic
	case "group":
		return kafka.ResourceTypeGroup
	case "cluster":
		return kafka.ResourceTypeCluster
	case "transactional_id":
		return kafka.ResourceTypeTransactionalID
	default:
		return kafka.ResourceTypeUnknown
	}
}

func resourceTypeToString(rt kafka.ResourceType) string {
	switch rt {
	case kafka.ResourceTypeTopic:
		return "topic"
	case kafka.ResourceTypeGroup:
		return "group"
	case kafka.ResourceTypeCluster:
		return "cluster"
	case kafka.ResourceTypeTransactionalID:
		return "transactional_id"
	default:
		return "unknown"
	}
}

// Pattern type conversions
func patternTypeFromString(s string) kafka.PatternType {
	switch s {
	case "literal":
		return kafka.PatternTypeLiteral
	case "prefixed":
		return kafka.PatternTypePrefixed
	default:
		return kafka.PatternTypeLiteral
	}
}

func patternTypeToString(pt kafka.PatternType) string {
	switch pt {
	case kafka.PatternTypeLiteral:
		return "literal"
	case kafka.PatternTypePrefixed:
		return "prefixed"
	default:
		return "literal"
	}
}

// Operation conversions
func operationFromString(s string) kafka.ACLOperationType {
	switch s {
	case "read":
		return kafka.ACLOperationTypeRead
	case "write":
		return kafka.ACLOperationTypeWrite
	case "create":
		return kafka.ACLOperationTypeCreate
	case "delete":
		return kafka.ACLOperationTypeDelete
	case "alter":
		return kafka.ACLOperationTypeAlter
	case "describe":
		return kafka.ACLOperationTypeDescribe
	case "all":
		return kafka.ACLOperationTypeAll
	default:
		return kafka.ACLOperationTypeUnknown
	}
}

func operationToString(op kafka.ACLOperationType) string {
	switch op {
	case kafka.ACLOperationTypeRead:
		return "read"
	case kafka.ACLOperationTypeWrite:
		return "write"
	case kafka.ACLOperationTypeCreate:
		return "create"
	case kafka.ACLOperationTypeDelete:
		return "delete"
	case kafka.ACLOperationTypeAlter:
		return "alter"
	case kafka.ACLOperationTypeDescribe:
		return "describe"
	case kafka.ACLOperationTypeAll:
		return "all"
	default:
		return "unknown"
	}
}

// Permission type conversions
func permissionTypeFromString(s string) kafka.ACLPermissionType {
	switch s {
	case "allow":
		return kafka.ACLPermissionTypeAllow
	case "deny":
		return kafka.ACLPermissionTypeDeny
	default:
		return kafka.ACLPermissionTypeUnknown
	}
}

func permissionTypeToString(pt kafka.ACLPermissionType) string {
	switch pt {
	case kafka.ACLPermissionTypeAllow:
		return "allow"
	case kafka.ACLPermissionTypeDeny:
		return "deny"
	default:
		return "unknown"
	}
}
