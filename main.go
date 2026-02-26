// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

// Terraform Provider for Streamline - Kafka-compatible streaming platform
//
// This provider enables Infrastructure as Code management of Streamline resources:
// - Topics (create, configure, delete)
// - ACLs (access control lists)
// - Schemas (schema registry management)
// - Consumer Groups (offset management)
//
// Usage:
//
//	terraform {
//	  required_providers {
//	    streamline = {
//	      source = "streamline-platform/streamline"
//	    }
//	  }
//	}
//
//	provider "streamline" {
//	  bootstrap_servers = "localhost:9092"
//	}

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name streamline

package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/streamline-platform/terraform-provider-streamline/internal/provider"
)

var (
	// Version is set during build
	version string = "dev"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/streamline-platform/streamline",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}

