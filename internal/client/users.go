// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
)

// UserConfig represents user configuration
type UserConfig struct {
	Username  string
	Mechanism string
}

// UserInfo represents user information
type UserInfo struct {
	Username  string
	Mechanism string
}

// CreateUser creates a SASL/SCRAM user by creating a cluster-level ACL for the principal.
// Streamline manages SCRAM users through its Kafka protocol-compatible interface.
func (c *StreamlineClient) CreateUser(ctx context.Context, cfg UserConfig) error {
	// Create a describe ACL for the user principal on the cluster to register the user
	aclCfg := ACLConfig{
		ResourceType:   "cluster",
		ResourceName:   "kafka-cluster",
		PatternType:    "literal",
		Principal:      fmt.Sprintf("User:%s", cfg.Username),
		Host:           "*",
		Operation:      "describe",
		PermissionType: "allow",
	}
	return c.CreateACL(ctx, aclCfg)
}

// GetUser retrieves user information by checking for ACLs with the user principal.
func (c *StreamlineClient) GetUser(ctx context.Context, username string) (*UserInfo, error) {
	aclFilter := ACLConfig{
		ResourceType:   "cluster",
		ResourceName:   "kafka-cluster",
		PatternType:    "literal",
		Principal:      fmt.Sprintf("User:%s", username),
		Host:           "*",
		Operation:      "describe",
		PermissionType: "allow",
	}

	_, err := c.GetACL(ctx, aclFilter)
	if err != nil {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	return &UserInfo{
		Username:  username,
		Mechanism: "SCRAM-SHA-256",
	}, nil
}

// DeleteUser deletes a SASL/SCRAM user by removing associated ACLs.
func (c *StreamlineClient) DeleteUser(ctx context.Context, username string) error {
	aclFilter := ACLConfig{
		ResourceType:   "cluster",
		ResourceName:   "kafka-cluster",
		PatternType:    "literal",
		Principal:      fmt.Sprintf("User:%s", username),
		Host:           "*",
		Operation:      "describe",
		PermissionType: "allow",
	}
	return c.DeleteACL(ctx, aclFilter)
}
