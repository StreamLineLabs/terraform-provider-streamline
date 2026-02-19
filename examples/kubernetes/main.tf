# Kubernetes Example - Streamline with Kubernetes Operator
#
# This example demonstrates deploying Streamline resources alongside
# Kubernetes-managed infrastructure using the Streamline operator.
# It provisions topics, users, and ACLs for a microservices architecture.

terraform {
  required_providers {
    streamline = {
      source  = "streamline-platform/streamline"
      version = "~> 1.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
  }
}

variable "streamline_endpoint" {
  type        = string
  default     = "streamline.streamline-system.svc.cluster.local:9092"
  description = "Streamline bootstrap servers endpoint within the Kubernetes cluster"
}

variable "namespace" {
  type        = string
  default     = "default"
  description = "Kubernetes namespace for application resources"
}

# Connect to Streamline running in Kubernetes
provider "streamline" {
  bootstrap_servers = var.streamline_endpoint
}

provider "kubernetes" {
  config_path = "~/.kube/config"
}

# ============================================================================
# Cluster Info
# ============================================================================

data "streamline_cluster" "current" {}

# ============================================================================
# Users for Microservices
# ============================================================================

resource "streamline_user" "order_service" {
  username  = "order-service"
  mechanism = "SCRAM-SHA-256"
}

resource "streamline_user" "payment_service" {
  username  = "payment-service"
  mechanism = "SCRAM-SHA-256"
}

resource "streamline_user" "notification_service" {
  username  = "notification-service"
  mechanism = "SCRAM-SHA-256"
}

# ============================================================================
# Topics
# ============================================================================

resource "streamline_topic" "orders" {
  name              = "orders"
  partitions        = 12
  replication_factor = 1

  config = {
    "retention.ms" = "604800000" # 7 days
  }
}

resource "streamline_topic" "payments" {
  name              = "payments"
  partitions        = 6
  replication_factor = 1

  config = {
    "retention.ms" = "604800000"
  }
}

resource "streamline_topic" "notifications" {
  name              = "notifications"
  partitions        = 3
  replication_factor = 1

  config = {
    "retention.ms" = "86400000" # 1 day
  }
}

resource "streamline_topic" "orders_dlq" {
  name              = "orders-dlq"
  partitions        = 3
  replication_factor = 1

  config = {
    "retention.ms" = "2592000000" # 30 days
  }
}

# ============================================================================
# ACLs - Order Service
# ============================================================================

resource "streamline_acl" "order_service_write_orders" {
  resource_type   = "topic"
  resource_name   = streamline_topic.orders.name
  principal       = "User:${streamline_user.order_service.username}"
  operation       = "write"
  permission_type = "allow"
}

resource "streamline_acl" "order_service_read_payments" {
  resource_type   = "topic"
  resource_name   = streamline_topic.payments.name
  principal       = "User:${streamline_user.order_service.username}"
  operation       = "read"
  permission_type = "allow"
}

# ============================================================================
# ACLs - Payment Service
# ============================================================================

resource "streamline_acl" "payment_service_read_orders" {
  resource_type   = "topic"
  resource_name   = streamline_topic.orders.name
  principal       = "User:${streamline_user.payment_service.username}"
  operation       = "read"
  permission_type = "allow"
}

resource "streamline_acl" "payment_service_write_payments" {
  resource_type   = "topic"
  resource_name   = streamline_topic.payments.name
  principal       = "User:${streamline_user.payment_service.username}"
  operation       = "write"
  permission_type = "allow"
}

# ============================================================================
# ACLs - Notification Service
# ============================================================================

resource "streamline_acl" "notification_service_read_orders" {
  resource_type   = "topic"
  resource_name   = streamline_topic.orders.name
  principal       = "User:${streamline_user.notification_service.username}"
  operation       = "read"
  permission_type = "allow"
}

resource "streamline_acl" "notification_service_write_notifications" {
  resource_type   = "topic"
  resource_name   = streamline_topic.notifications.name
  principal       = "User:${streamline_user.notification_service.username}"
  operation       = "write"
  permission_type = "allow"
}

# ============================================================================
# Kubernetes ConfigMap with topic configuration
# ============================================================================

resource "kubernetes_config_map" "streamline_config" {
  metadata {
    name      = "streamline-topics"
    namespace = var.namespace
  }

  data = {
    STREAMLINE_BOOTSTRAP_SERVERS = var.streamline_endpoint
    ORDERS_TOPIC                 = streamline_topic.orders.name
    PAYMENTS_TOPIC               = streamline_topic.payments.name
    NOTIFICATIONS_TOPIC          = streamline_topic.notifications.name
    ORDERS_DLQ_TOPIC             = streamline_topic.orders_dlq.name
  }
}

# ============================================================================
# Outputs
# ============================================================================

output "cluster_id" {
  value = data.streamline_cluster.current.cluster_id
}

output "broker_count" {
  value = length(data.streamline_cluster.current.brokers)
}

output "topics" {
  value = {
    orders        = streamline_topic.orders.name
    payments      = streamline_topic.payments.name
    notifications = streamline_topic.notifications.name
    orders_dlq    = streamline_topic.orders_dlq.name
  }
}

output "users" {
  value = [
    streamline_user.order_service.username,
    streamline_user.payment_service.username,
    streamline_user.notification_service.username,
  ]
}
