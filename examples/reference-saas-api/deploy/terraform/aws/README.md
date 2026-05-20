# AWS Terraform Starter

This starter creates dependency primitives for the generated service: RDS Postgres, ElastiCache Redis, an S3 bucket, IAM policy examples, and outputs that can be copied into Helm values or your deployment pipeline.

It intentionally avoids choosing ECS or EKS application hosting. Wire these outputs into the platform you already operate.
