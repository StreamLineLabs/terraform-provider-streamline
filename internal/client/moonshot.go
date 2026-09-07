// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"net/http"
	"strings"
	"time"
)

// MoonshotConfig configures the reserved Moonshot HTTP client. Terraform
// resources do not currently issue Moonshot requests because their legacy
// schemas do not match provisionable broker objects.
type MoonshotConfig struct {
	BaseURL string
	Token   string
	Timeout time.Duration
}

// MoonshotClient retains validated connection configuration for a future,
// versioned resource model.
type MoonshotClient struct {
	base    string
	token   string
	httpCli *http.Client
}

func NewMoonshotClient(cfg MoonshotConfig) *MoonshotClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &MoonshotClient{
		base:    strings.TrimRight(cfg.BaseURL, "/"),
		token:   cfg.Token,
		httpCli: &http.Client{Timeout: timeout},
	}
}
