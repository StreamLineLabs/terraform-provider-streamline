# Production Example - Enterprise Setup with Auth and ACLs
#
# This example demonstrates a production setup with:
# - SASL authentication
# - TLS encryption
# - Fine-grained ACLs
# - Schema Registry

terraform {
  required_providers {
    streamline = {
      source  = "streamline-platform/streamline"
      version = "~> 1.0"
    }
  }
}

variable "bootstrap_servers" {
  type        = string
  description = "Streamline bootstrap servers"
}

variable "sasl_username" {
  type        = string
  description = "SASL username"
}

variable "sasl_password" {
  type        = string
  sensitive   = true
  description = "SASL password"
}

# Connect with authentication
provider "streamline" {
  bootstrap_servers = var.bootstrap_servers
  sasl_mechanism    = "SCRAM-SHA-256"
  sasl_username     = var.sasl_username
  sasl_password     = var.sasl_password
  tls_enabled       = true
}

# ============================================================================
# Topics
# ============================================================================

resource "streamline_topic" "events" {
  name              = "production-events"
  partitions        = 12
  replication_factor = 3

  config = {
    "retention.ms"       = "604800000"  # 7 days
    "min.insync.replicas" = "2"
  }
}

resource "streamline_topic" "events_dlq" {
  name              = "production-events-dlq"
  partitions        = 6
  replication_factor = 3

  config = {
    "retention.ms" = "2592000000" # 30 days
  }
}

resource "streamline_topic" "audit_log" {
  name              = "audit-log"
  partitions        = 6
  replication_factor = 3
  cleanup_policy    = "delete"

  config = {
    "retention.ms" = "7776000000" # 90 days
  }
}

# ============================================================================
# ACLs - Producer Service
# ============================================================================

resource "streamline_acl" "producer_write_events" {
  resource_type   = "topic"
  resource_name   = streamline_topic.events.name
  pattern_type    = "literal"
  principal       = "User:producer-service"
  operation       = "write"
  permission_type = "allow"
}

resource "streamline_acl" "producer_describe_events" {
  resource_type   = "topic"
  resource_name   = streamline_topic.events.name
  pattern_type    = "literal"
  principal       = "User:producer-service"
  operation       = "describe"
  permission_type = "allow"
}

# ============================================================================
# ACLs - Consumer Service
# ============================================================================

resource "streamline_acl" "consumer_read_events" {
  resource_type   = "topic"
  resource_name   = streamline_topic.events.name
  pattern_type    = "literal"
  principal       = "User:consumer-service"
  operation       = "read"
  permission_type = "allow"
}

resource "streamline_acl" "consumer_group" {
  resource_type   = "group"
  resource_name   = "consumer-group-1"
  pattern_type    = "literal"
  principal       = "User:consumer-service"
  operation       = "read"
  permission_type = "allow"
}

# ============================================================================
# Schemas
# ============================================================================

resource "streamline_schema" "events_value" {
  subject     = "${streamline_topic.events.name}-value"
  schema_type = "AVRO"

  schema = jsonencode({
    type      = "record"
    name      = "Event"
    namespace = "com.example.events"
    fields = [
      { name = "id", type = "string" },
      { name = "type", type = "string" },
      { name = "timestamp", type = "long" },
      { name = "payload", type = { type = "map", values = "string" } }
    ]
  })

  compatibility = "BACKWARD"
}

# ============================================================================
# Outputs
# ============================================================================

output "events_topic" {
  value = streamline_topic.events.name
}

output "dlq_topic" {
  value = streamline_topic.events_dlq.name
}

output "schema_id" {
  value = streamline_schema.events_value.schema_id
}
