# IAM Module for EC2 Instance Role
resource "aws_iam_role" "this" {
  name = var.role_name

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })

  tags = merge(
    var.tags,
    {
      Name = var.role_name
    }
  )
}

# DynamoDB Policy
resource "aws_iam_policy" "dynamodb" {
  name        = "${var.role_name}-dynamodb-policy"
  description = "Policy for DynamoDB access"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:UpdateItem",
          "dynamodb:DeleteItem",
          "dynamodb:Query",
          "dynamodb:Scan",
          "dynamodb:BatchGetItem",
          "dynamodb:BatchWriteItem",
          "dynamodb:DescribeTable"
        ]
        Resource = concat(
          var.dynamodb_table_arns,
          [for arn in var.dynamodb_table_arns : "${arn}/index/*"]
        )
      }
    ]
  })
}

# S3 Policy
resource "aws_iam_policy" "s3" {
  name        = "${var.role_name}-s3-policy"
  description = "Policy for S3 bucket access"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:ListBucket",
          "s3:GetBucketLocation",
          "s3:ListBucketMultipartUploads",
          "s3:AbortMultipartUpload"
        ]
        Resource = concat(
          var.s3_bucket_arns,
          [for arn in var.s3_bucket_arns : "${arn}/*"]
        )
      }
    ]
  })
}

# ECR Policy. ecr:GetAuthorizationToken does not support resource-level
# permissions and must remain on "*". The pull actions are scoped to
# var.ecr_repository_arns when provided; otherwise they fall back to "*" for
# backwards compatibility with environments that have not yet wired the ARN.
resource "aws_iam_policy" "ecr" {
  name        = "${var.role_name}-ecr-policy"
  description = "Policy for ECR access"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "EcrAuth"
        Effect   = "Allow"
        Action   = ["ecr:GetAuthorizationToken"]
        Resource = "*"
      },
      {
        Sid    = "EcrPull"
        Effect = "Allow"
        Action = [
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage"
        ]
        Resource = length(var.ecr_repository_arns) > 0 ? var.ecr_repository_arns : ["*"]
      }
    ]
  })
}

# SSM Parameter Store policy (optional). Scoped to a single path prefix so the
# instance role can only read its own environment's parameters.
resource "aws_iam_policy" "ssm_parameters" {
  count       = var.ssm_parameter_path != "" ? 1 : 0
  name        = "${var.role_name}-ssm-parameters-policy"
  description = "Read-only access to SSM parameters under ${var.ssm_parameter_path}"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ssm:GetParameter",
          "ssm:GetParameters",
          "ssm:GetParametersByPath",
        ]
        Resource = "arn:aws:ssm:${var.aws_region}:*:parameter${var.ssm_parameter_path}/*"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "ssm_parameters" {
  count      = var.ssm_parameter_path != "" ? 1 : 0
  role       = aws_iam_role.this.name
  policy_arn = aws_iam_policy.ssm_parameters[0].arn
}

# CloudWatch Logs Policy
resource "aws_iam_policy" "cloudwatch" {
  name        = "${var.role_name}-cloudwatch-policy"
  description = "Policy for CloudWatch Logs access"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:DescribeLogStreams"
        ]
        Resource = "arn:aws:logs:${var.aws_region}:*:log-group:/aws/${var.project_name}/*"
      }
    ]
  })
}

# OpenSearch Policy (conditional)
resource "aws_iam_policy" "opensearch" {
  count       = length(var.opensearch_domain_arns) > 0 ? 1 : 0
  name        = "${var.role_name}-opensearch-policy"
  description = "Policy for OpenSearch HTTP access"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["es:ESHttpGet", "es:ESHttpPut", "es:ESHttpPost", "es:ESHttpDelete", "es:ESHttpHead"]
        Resource = [for arn in var.opensearch_domain_arns : "${arn}/*"]
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "opensearch" {
  count      = length(var.opensearch_domain_arns) > 0 ? 1 : 0
  role       = aws_iam_role.this.name
  policy_arn = aws_iam_policy.opensearch[0].arn
}

# Attach Policies
resource "aws_iam_role_policy_attachment" "dynamodb" {
  role       = aws_iam_role.this.name
  policy_arn = aws_iam_policy.dynamodb.arn
}

resource "aws_iam_role_policy_attachment" "s3" {
  role       = aws_iam_role.this.name
  policy_arn = aws_iam_policy.s3.arn
}

resource "aws_iam_role_policy_attachment" "ecr" {
  role       = aws_iam_role.this.name
  policy_arn = aws_iam_policy.ecr.arn
}

resource "aws_iam_role_policy_attachment" "cloudwatch" {
  role       = aws_iam_role.this.name
  policy_arn = aws_iam_policy.cloudwatch.arn
}

resource "aws_iam_role_policy_attachment" "ssm" {
  role       = aws_iam_role.this.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

# Instance Profile
resource "aws_iam_instance_profile" "this" {
  name = "${var.role_name}-profile"
  role = aws_iam_role.this.name

  tags = merge(
    var.tags,
    {
      Name = "${var.role_name}-profile"
    }
  )
}
