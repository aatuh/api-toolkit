output "database_endpoint" {
  value = aws_db_instance.postgres.address
}

output "redis_endpoint" {
  value = aws_elasticache_replication_group.redis.primary_endpoint_address
}

output "object_bucket_name" {
  value = aws_s3_bucket.objects.bucket
}

output "service_policy_arn" {
  value = aws_iam_policy.service.arn
}
