# Basic Example - Local Development Setup
#
# This example demonstrates basic usage of the Streamline Terraform provider
# for local development.

terraform {
  required_providers {
    streamline = {
      source  = "streamline-platform/streamline"
      version = "~> 1.0"
    }
  }
}

# Connect to local Streamline instance
provider "streamline" {
  bootstrap_servers = "localhost:9092"
}

# Create topics for an event-driven application
resource "streamline_topic" "user_events" {
  name       = "user-events"
  partitions = 3

  config = {
    "retention.ms" = "604800000" # 7 days
  }
}

resource "streamline_topic" "order_events" {
  name       = "order-events"
  partitions = 6

  config = {
    "retention.ms" = "2592000000" # 30 days
  }
}

resource "streamline_topic" "user_state" {
  name           = "user-state"
  partitions     = 12
  cleanup_policy = "compact"
  retention_ms   = -1 # Infinite for compacted topics
}

# Create a user for the application
resource "streamline_user" "app" {
  username  = "my-application"
  mechanism = "SCRAM-SHA-256"
}

# Output topic names
output "user_events_topic" {
  value = streamline_topic.user_events.name
}

output "order_events_topic" {
  value = streamline_topic.order_events.name
}
