resource "aws_db_instance" "postgres" {
  identifier           = var.name
  engine               = "postgres"
  instance_class       = var.postgres_instance_class
  allocated_storage    = 20
  username             = var.postgres_username
  password             = var.postgres_password
  skip_final_snapshot  = true
}

resource "aws_elasticache_replication_group" "redis" {
  replication_group_id = "${var.name}-redis"
  description          = "Redis for api-toolkit generated services"
  node_type            = var.redis_node_type
  num_cache_clusters   = 1
}

resource "aws_s3_bucket" "objects" {
  bucket = var.object_bucket_name
}

resource "aws_iam_policy" "service" {
  name   = "${var.name}-service"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
        Resource = "${aws_s3_bucket.objects.arn}/*"
      }
    ]
  })
}
