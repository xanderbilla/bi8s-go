# Bootstrap - Creates S3 bucket and DynamoDB table for Terraform state management
# Run this first before deploying any environment

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# S3 Bucket for Terraform State
resource "aws_s3_bucket" "terraform_state" {
  bucket = "${var.project_name}-terraform-state-${var.environment}"

  tags = {
    Name        = "${var.project_name}-terraform-state-${var.environment}"
    Environment = var.environment
    Project     = var.project_name
    ManagedBy   = "OpenTofu"
  }
}

# Enable Versioning
resource "aws_s3_bucket_versioning" "terraform_state" {
  bucket = aws_s3_bucket.terraform_state.id

  versioning_configuration {
    status = "Enabled"
  }
}

# Enable Encryption
resource "aws_s3_bucket_server_side_encryption_configuration" "terraform_state" {
  bucket = aws_s3_bucket.terraform_state.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# Block Public Access
resource "aws_s3_bucket_public_access_block" "terraform_state" {
  bucket = aws_s3_bucket.terraform_state.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Lifecycle Policy
resource "aws_s3_bucket_lifecycle_configuration" "terraform_state" {
  bucket = aws_s3_bucket.terraform_state.id

  rule {
    id     = "delete-old-versions"
    status = "Enabled"

    noncurrent_version_expiration {
      noncurrent_days = 90
    }
  }
}

# DynamoDB Table for State Locking
resource "aws_dynamodb_table" "terraform_locks" {
  name         = "${var.project_name}-terraform-locks-${var.environment}"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "LockID"

  attribute {
    name = "LockID"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }

  tags = {
    Name        = "${var.project_name}-terraform-locks-${var.environment}"
    Environment = var.environment
    Project     = var.project_name
    ManagedBy   = "OpenTofu"
  }
}

# Outputs
output "state_bucket_name" {
  description = "S3 bucket name for Terraform state"
  value       = aws_s3_bucket.terraform_state.id
}

output "lock_table_name" {
  description = "DynamoDB table name for state locking"
  value       = aws_dynamodb_table.terraform_locks.name
}

output "backend_config" {
  description = "Backend configuration for Terraform"
  value       = <<-EOT
    bucket         = "${aws_s3_bucket.terraform_state.id}"
    key            = "${var.project_name}/${var.environment}/terraform.tfstate"
    region         = "${var.aws_region}"
    dynamodb_table = "${aws_dynamodb_table.terraform_locks.name}"
    encrypt        = true
  EOT
}

# ---------------------------------------------------------------------------
# GitHub Actions OIDC deploy role for this environment
# ---------------------------------------------------------------------------
#
# Branch trust: workflows on the matching branch (dev branch -> dev role,
# prod branch -> prod role) may assume this role.
# Environment trust: workflows running under the matching GitHub Environment
# (configure required reviewers on the prod environment) may assume it.
# This eliminates the need for static AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY
# secrets in the repo (B4) and gates prod applies behind GitHub's environment
# approval mechanism (B5).

module "github_oidc" {
  source = "../modules/github-oidc"

  github_owner         = var.github_owner
  github_repo          = var.github_repo
  role_name            = "${var.project_name}-gha-deploy-${var.environment}"
  allowed_branches     = [var.environment]
  allowed_environments = [var.environment]
  allow_pull_requests  = var.allow_pr_plan
  create_oidc_provider = var.create_oidc_provider

  tags = {
    Name        = "${var.project_name}-gha-deploy-${var.environment}"
    Environment = var.environment
    Project     = var.project_name
    ManagedBy   = "OpenTofu"
  }
}

output "github_oidc_role_arn" {
  description = "ARN of the GitHub Actions deploy role for this environment. Set as the AWS_OIDC_ROLE_ARN secret in the matching GitHub Environment."
  value       = module.github_oidc.role_arn
}

output "github_oidc_role_name" {
  description = "Name of the GitHub Actions deploy role"
  value       = module.github_oidc.role_name
}

output "github_oidc_provider_arn" {
  description = "ARN of the OIDC provider"
  value       = module.github_oidc.oidc_provider_arn
}
