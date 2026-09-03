// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	if err := ValidateManagedACLConfig(cfg); err != nil {
		return err
	}
	entry, err := aclEntryFromConfig(cfg)
	if err != nil {
		return err
	}

	return c.withRetry(ctx, "create ACL", func(ctx context.Context) error {
		resp, err := c.kafkaClient.CreateACLs(ctx, &kafka.CreateACLsRequest{
			ACLs: []kafka.ACLEntry{entry},
		})
		if err != nil {
			return err
		}
		return validateCreateACLResults(resp.Errors)
	})
}

func validateCreateACLResults(results []error) error {
	if len(results) != 1 {
		return fmt.Errorf(
			"broker returned %d ACL creation results for one requested ACL; ACL support may be disabled",
			len(results),
		)
	}
	if results[0] != nil {
		return fmt.Errorf("ACL 0 was rejected: %w", results[0])
	}
	return nil
}

// GetACL retrieves a single ACL entry matching the filter
func (c *StreamlineClient) GetACL(ctx context.Context, cfg ACLConfig) (*ACLConfig, error) {
	filter, err := aclFilterFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	resp, err := c.kafkaClient.DescribeACLs(ctx, &kafka.DescribeACLsRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ACLs: %w", err)
	}

	if resp.Error != nil {
		if errors.Is(resp.Error, kafka.ResourceNotFound) {
			return nil, NewNotFoundError("ACL", aclIdentifier(cfg), resp.Error)
		}
		return nil, fmt.Errorf("failed to describe ACLs: %w", resp.Error)
	}

	if len(resp.Resources) == 0 {
		return nil, NewNotFoundError("ACL", aclIdentifier(cfg), nil)
	}

	// The broker may omit unsupported filter fields. Never accept a broader
	// response as the managed ACL.
	for _, res := range resp.Resources {
		for _, description := range res.ACLs {
			result, err := aclConfigFromResponse(res, description)
			if err != nil {
				return nil, fmt.Errorf("failed to decode ACL response: %w", err)
			}
			if aclConfigsEqual(cfg, *result) {
				return result, nil
			}
		}
	}

	return nil, NewNotFoundError("ACL", aclIdentifier(cfg), nil)
}

// DeleteACL deletes ACL entries matching the filter
func (c *StreamlineClient) DeleteACL(ctx context.Context, cfg ACLConfig) error {
	if err := ValidateManagedACLConfig(cfg); err != nil {
		return err
	}
	filter, err := deleteACLFilterFromConfig(cfg)
	if err != nil {
		return err
	}

	return c.withRetry(ctx, "delete ACL", func(ctx context.Context) error {
		resp, err := c.kafkaClient.DeleteACLs(ctx, &kafka.DeleteACLsRequest{
			Filters: []kafka.DeleteACLsFilter{filter},
		})
		if err != nil {
			return err
		}
		return validateDeleteACLResults(cfg, resp.Results)
	})
}

func validateDeleteACLResults(cfg ACLConfig, results []kafka.DeleteACLsResult) error {
	matched := false
	for i, result := range results {
		if result.Error != nil {
			if errors.Is(result.Error, kafka.ResourceNotFound) {
				return NewNotFoundError("ACL", aclIdentifier(cfg), result.Error)
			}
			return fmt.Errorf("ACL filter %d was rejected: %w", i, result.Error)
		}
		for j, matchingACL := range result.MatchingACLs {
			if matchingACL.Error != nil {
				return fmt.Errorf("ACL filter %d matching entry %d failed: %w", i, j, matchingACL.Error)
			}
			decoded, err := aclConfigFromDeleteResponse(matchingACL)
			if err != nil {
				return fmt.Errorf("ACL filter %d matching entry %d could not be decoded: %w", i, j, err)
			}
			if !aclConfigsEqual(cfg, decoded) {
				return fmt.Errorf(
					"ACL filter %d unexpectedly matched %s; refusing to report an exact deletion",
					i,
					aclIdentifier(decoded),
				)
			}
			matched = true
		}
	}
	if !matched {
		return NewNotFoundError("ACL", aclIdentifier(cfg), nil)
	}
	return nil
}

func aclIdentifier(cfg ACLConfig) string {
	return fmt.Sprintf(
		"%s/%s/%s/%s/%s/%s/%s",
		cfg.ResourceType,
		cfg.PatternType,
		cfg.ResourceName,
		cfg.Principal,
		cfg.Host,
		cfg.Operation,
		cfg.PermissionType,
	)
}

func aclEntryFromConfig(cfg ACLConfig) (kafka.ACLEntry, error) {
	for field, value := range map[string]string{
		"resource_name": cfg.ResourceName,
		"principal":     cfg.Principal,
		"host":          cfg.Host,
	} {
		if strings.TrimSpace(value) == "" {
			return kafka.ACLEntry{}, fmt.Errorf("ACL %s must not be empty", field)
		}
	}
	if !strings.HasPrefix(cfg.Principal, "User:") && !strings.HasPrefix(cfg.Principal, "Group:") {
		return kafka.ACLEntry{}, fmt.Errorf("ACL principal must use the form User:name or Group:name")
	}
	if cfg.Principal == "User:" || cfg.Principal == "Group:" {
		return kafka.ACLEntry{}, fmt.Errorf("ACL principal name must not be empty")
	}

	resourceType, err := resourceTypeFromString(cfg.ResourceType)
	if err != nil {
		return kafka.ACLEntry{}, err
	}
	patternType, err := creationPatternTypeFromString(cfg.PatternType)
	if err != nil {
		return kafka.ACLEntry{}, err
	}
	operation, err := operationFromString(cfg.Operation)
	if err != nil {
		return kafka.ACLEntry{}, err
	}
	permissionType, err := permissionTypeFromString(cfg.PermissionType)
	if err != nil {
		return kafka.ACLEntry{}, err
	}

	return kafka.ACLEntry{
		ResourceType:        resourceType,
		ResourceName:        cfg.ResourceName,
		ResourcePatternType: patternType,
		Principal:           cfg.Principal,
		Host:                cfg.Host,
		Operation:           operation,
		PermissionType:      permissionType,
	}, nil
}

// ValidateACLConfig verifies that an ACL can be represented exactly by the
// Kafka API used by this provider.
func ValidateACLConfig(cfg ACLConfig) error {
	if _, err := aclEntryFromConfig(cfg); err != nil {
		return err
	}
	if _, err := filterPatternTypeFromString(cfg.PatternType); err != nil {
		return err
	}
	return nil
}

// ValidateManagedACLConfig rejects values that Streamline 0.3.0 cannot delete
// exactly. Legacy broad ACLs can still be described, but must be cleaned up
// outside this provider.
func ValidateManagedACLConfig(cfg ACLConfig) error {
	if err := ValidateACLConfig(cfg); err != nil {
		return err
	}
	if cfg.ResourceType == "delegation_token" {
		return fmt.Errorf("ACL resource type %q is not supported by Streamline 0.3.0", cfg.ResourceType)
	}
	if cfg.ResourceName == "*" {
		return fmt.Errorf("ACL resource_name %q cannot be managed safely because the broker treats it as a delete-filter wildcard", cfg.ResourceName)
	}
	switch cfg.Principal {
	case "*", "User:*", "Group:*":
		return fmt.Errorf("ACL principal %q cannot be managed safely because the broker treats it as a delete-filter wildcard", cfg.Principal)
	}
	if cfg.Host == "*" {
		return fmt.Errorf("ACL host %q cannot be managed safely because the broker treats it as a delete-filter wildcard", cfg.Host)
	}
	if cfg.Operation == "all" {
		return fmt.Errorf("ACL operation %q cannot be managed safely because the broker cannot delete it exactly", cfg.Operation)
	}
	return nil
}

func aclFilterFromConfig(cfg ACLConfig) (kafka.ACLFilter, error) {
	entry, err := aclEntryFromConfig(cfg)
	if err != nil {
		return kafka.ACLFilter{}, err
	}
	patternType, err := filterPatternTypeFromString(cfg.PatternType)
	if err != nil {
		return kafka.ACLFilter{}, err
	}
	return kafka.ACLFilter{
		ResourceTypeFilter:        entry.ResourceType,
		ResourceNameFilter:        entry.ResourceName,
		ResourcePatternTypeFilter: patternType,
		PrincipalFilter:           entry.Principal,
		HostFilter:                entry.Host,
		Operation:                 entry.Operation,
		PermissionType:            entry.PermissionType,
	}, nil
}

func deleteACLFilterFromConfig(cfg ACLConfig) (kafka.DeleteACLsFilter, error) {
	entry, err := aclEntryFromConfig(cfg)
	if err != nil {
		return kafka.DeleteACLsFilter{}, err
	}
	patternType, err := filterPatternTypeFromString(cfg.PatternType)
	if err != nil {
		return kafka.DeleteACLsFilter{}, err
	}
	return kafka.DeleteACLsFilter{
		ResourceTypeFilter:        entry.ResourceType,
		ResourceNameFilter:        entry.ResourceName,
		ResourcePatternTypeFilter: patternType,
		PrincipalFilter:           entry.Principal,
		HostFilter:                entry.Host,
		Operation:                 entry.Operation,
		PermissionType:            entry.PermissionType,
	}, nil
}

func aclConfigFromResponse(resource kafka.ACLResource, acl kafka.ACLDescription) (*ACLConfig, error) {
	resourceType, err := resourceTypeToString(resource.ResourceType)
	if err != nil {
		return nil, err
	}
	patternType, err := patternTypeToString(resource.PatternType)
	if err != nil {
		return nil, err
	}
	operation, err := operationToString(acl.Operation)
	if err != nil {
		return nil, err
	}
	permissionType, err := permissionTypeToString(acl.PermissionType)
	if err != nil {
		return nil, err
	}
	return &ACLConfig{
		ResourceType:   resourceType,
		ResourceName:   resource.ResourceName,
		PatternType:    patternType,
		Principal:      acl.Principal,
		Host:           acl.Host,
		Operation:      operation,
		PermissionType: permissionType,
	}, nil
}

func aclConfigFromDeleteResponse(acl kafka.DeleteACLsMatchingACLs) (ACLConfig, error) {
	resourceType, err := resourceTypeToString(acl.ResourceType)
	if err != nil {
		return ACLConfig{}, err
	}
	patternType, err := patternTypeToString(acl.ResourcePatternType)
	if err != nil {
		return ACLConfig{}, err
	}
	operation, err := operationToString(acl.Operation)
	if err != nil {
		return ACLConfig{}, err
	}
	permissionType, err := permissionTypeToString(acl.PermissionType)
	if err != nil {
		return ACLConfig{}, err
	}
	return ACLConfig{
		ResourceType:   resourceType,
		ResourceName:   acl.ResourceName,
		PatternType:    patternType,
		Principal:      acl.Principal,
		Host:           acl.Host,
		Operation:      operation,
		PermissionType: permissionType,
	}, nil
}

func aclConfigsEqual(left, right ACLConfig) bool {
	return left.ResourceType == right.ResourceType &&
		left.ResourceName == right.ResourceName &&
		left.PatternType == right.PatternType &&
		left.Principal == right.Principal &&
		left.Host == right.Host &&
		left.Operation == right.Operation &&
		left.PermissionType == right.PermissionType
}

// Resource type conversions.
func resourceTypeFromString(s string) (kafka.ResourceType, error) {
	switch s {
	case "topic":
		return kafka.ResourceTypeTopic, nil
	case "group":
		return kafka.ResourceTypeGroup, nil
	case "cluster":
		return kafka.ResourceTypeCluster, nil
	case "transactional_id":
		return kafka.ResourceTypeTransactionalID, nil
	case "delegation_token":
		return kafka.ResourceTypeDelegationToken, nil
	default:
		return kafka.ResourceTypeUnknown, fmt.Errorf("unsupported ACL resource type %q", s)
	}
}

func resourceTypeToString(rt kafka.ResourceType) (string, error) {
	switch rt {
	case kafka.ResourceTypeTopic:
		return "topic", nil
	case kafka.ResourceTypeGroup:
		return "group", nil
	case kafka.ResourceTypeCluster:
		return "cluster", nil
	case kafka.ResourceTypeTransactionalID:
		return "transactional_id", nil
	case kafka.ResourceTypeDelegationToken:
		return "delegation_token", nil
	default:
		return "", fmt.Errorf("unsupported Kafka ACL resource type %d", rt)
	}
}

// Streamline v0.3.0's ACL decoder interprets Kafka creation pattern codes 3
// and 4 in reverse. These creation values deliberately compensate for that
// server contract. Response values use the standard Kafka encoding.
func creationPatternTypeFromString(s string) (kafka.PatternType, error) {
	switch s {
	case "literal":
		return kafka.PatternTypePrefixed, nil
	case "prefixed":
		return kafka.PatternTypeLiteral, nil
	default:
		return kafka.PatternTypeUnknown, fmt.Errorf("unsupported ACL pattern type %q", s)
	}
}

// Streamline v0.3.0 decodes filter code 2 as literal and code 3 as prefixed.
// Code 4 is ignored, which would make deletion dangerously broad.
func filterPatternTypeFromString(s string) (kafka.PatternType, error) {
	switch s {
	case "literal":
		return kafka.PatternTypeMatch, nil
	case "prefixed":
		return kafka.PatternTypeLiteral, nil
	default:
		return kafka.PatternTypeUnknown, fmt.Errorf("unsupported ACL pattern type %q", s)
	}
}

func patternTypeToString(pt kafka.PatternType) (string, error) {
	switch pt {
	case kafka.PatternTypeLiteral:
		return "literal", nil
	case kafka.PatternTypePrefixed:
		return "prefixed", nil
	default:
		return "", fmt.Errorf("unsupported Kafka ACL pattern type %d", pt)
	}
}

// Operation conversions.
func operationFromString(s string) (kafka.ACLOperationType, error) {
	switch s {
	case "read":
		return kafka.ACLOperationTypeRead, nil
	case "write":
		return kafka.ACLOperationTypeWrite, nil
	case "create":
		return kafka.ACLOperationTypeCreate, nil
	case "delete":
		return kafka.ACLOperationTypeDelete, nil
	case "alter":
		return kafka.ACLOperationTypeAlter, nil
	case "describe":
		return kafka.ACLOperationTypeDescribe, nil
	case "all":
		return kafka.ACLOperationTypeAll, nil
	case "cluster_action":
		return kafka.ACLOperationTypeClusterAction, nil
	case "describe_configs":
		return kafka.ACLOperationTypeDescribeConfigs, nil
	case "alter_configs":
		return kafka.ACLOperationTypeAlterConfigs, nil
	case "idempotent_write":
		return kafka.ACLOperationTypeIdempotentWrite, nil
	default:
		return kafka.ACLOperationTypeUnknown, fmt.Errorf("unsupported ACL operation %q", s)
	}
}

func operationToString(op kafka.ACLOperationType) (string, error) {
	switch op {
	case kafka.ACLOperationTypeRead:
		return "read", nil
	case kafka.ACLOperationTypeWrite:
		return "write", nil
	case kafka.ACLOperationTypeCreate:
		return "create", nil
	case kafka.ACLOperationTypeDelete:
		return "delete", nil
	case kafka.ACLOperationTypeAlter:
		return "alter", nil
	case kafka.ACLOperationTypeDescribe:
		return "describe", nil
	case kafka.ACLOperationTypeAll:
		return "all", nil
	case kafka.ACLOperationTypeClusterAction:
		return "cluster_action", nil
	case kafka.ACLOperationTypeDescribeConfigs:
		return "describe_configs", nil
	case kafka.ACLOperationTypeAlterConfigs:
		return "alter_configs", nil
	case kafka.ACLOperationTypeIdempotentWrite:
		return "idempotent_write", nil
	default:
		return "", fmt.Errorf("unsupported Kafka ACL operation %d", op)
	}
}

// Permission type conversions.
func permissionTypeFromString(s string) (kafka.ACLPermissionType, error) {
	switch s {
	case "allow":
		return kafka.ACLPermissionTypeAllow, nil
	case "deny":
		return kafka.ACLPermissionTypeDeny, nil
	default:
		return kafka.ACLPermissionTypeUnknown, fmt.Errorf("unsupported ACL permission type %q", s)
	}
}

func permissionTypeToString(pt kafka.ACLPermissionType) (string, error) {
	switch pt {
	case kafka.ACLPermissionTypeAllow:
		return "allow", nil
	case kafka.ACLPermissionTypeDeny:
		return "deny", nil
	default:
		return "", fmt.Errorf("unsupported Kafka ACL permission type %d", pt)
	}
}
