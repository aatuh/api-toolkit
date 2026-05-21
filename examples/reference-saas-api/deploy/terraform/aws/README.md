# AWS Terraform Starter

This starter creates dependency primitives for the generated service: RDS
Postgres, ElastiCache Redis, an S3 bucket, IAM policy examples, and outputs
that can be copied into Helm values or your deployment pipeline.

It intentionally avoids choosing ECS or EKS application hosting. Wire these
outputs into the platform you already operate.

## Prerequisites

- Terraform with an AWS provider configuration supplied by the caller.
- A remote state backend configured by the application team before shared use.
- Network, subnet, security-group, encryption, backup, and access-review
  decisions owned by the deployment platform.

## Required Inputs

| Variable | Purpose |
| --- | --- |
| `name` | Prefix for generated dependency names. |
| `postgres_instance_class` | Starter RDS instance class. |
| `postgres_username` | Database admin username. |
| `postgres_password` | Sensitive database password; pass through a secret mechanism, not a committed tfvars file. |
| `redis_node_type` | Starter Redis node type. |
| `object_bucket_name` | S3 bucket name for object storage. |

## Outputs To Wire Into The Service

| Output | Service configuration |
| --- | --- |
| `database_endpoint` | Build the generated service `DATABASE_URL` secret. |
| `redis_endpoint` | Set `REDIS_ADDR` when Redis stores are enabled. |
| `object_bucket_name` | Set `S3_BUCKET` when `OBJECT_STORE=s3`. |
| `service_policy_arn` | Attach least-privilege object-store access to the runtime identity. |

## Validate

```sh
terraform fmt -check
terraform validate
terraform plan -out=tfplan
```

Record the plan output location and reviewed inputs in deployment evidence. Do
not paste secrets or full provider state into tickets, logs, or release notes.

## Non-goals

This starter does not choose the application host, Kubernetes cluster, ingress,
DNS, certificate, secret manager, VPC topology, backup policy, or production
sizing.
