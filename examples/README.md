# Terraform Provider Examples

## Examples

| Example | Description | Auth | Use Case |
|---------|-------------|------|----------|
| [basic](./basic/) | Local development setup | None | Getting started, local dev |
| [production](./production/) | Enterprise setup with auth, ACLs, schemas | SASL + TLS | Production deployments |
| [kubernetes](./kubernetes/) | Kubernetes integration with operator | In-cluster | K8s-native microservices |

## Usage

```bash
cd basic/  # or production/ or kubernetes/

# Copy and edit variables
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your values

# Initialize and apply
terraform init
terraform plan
terraform apply
```

## Prerequisites

- [Terraform](https://www.terraform.io/downloads) >= 1.0
- A running Streamline server
- For production example: SASL credentials and TLS certificates
- For kubernetes example: kubectl configured with cluster access
