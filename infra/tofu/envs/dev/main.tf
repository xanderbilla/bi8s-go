# IMDSv2 fix applied - instance will be recreated
terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "s3" {
    # Configuration provided via backend config file
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = local.common_tags
  }
}

locals {
  name_prefix = "${var.project_name}-${var.environment}"

  # Resource names
  dynamodb_movie_table     = "${var.project_name}-content-table-${var.environment}"
  dynamodb_person_table    = "${var.project_name}-person-table-${var.environment}"
  dynamodb_attribute_table = "${var.project_name}-attributes-table-${var.environment}"
  dynamodb_encoder_table   = "${var.project_name}-video-table-${var.environment}"
  s3_bucket                = "${var.project_name}-storage-${var.environment}"

  common_tags = merge(
    var.tags,
    {
      Project     = var.project_name
      Environment = var.environment
      ManagedBy   = "OpenTofu"
    }
  )
}

# VPC Module
module "vpc" {
  source = "../../modules/vpc"

  vpc_name             = "${local.name_prefix}-vpc"
  vpc_cidr             = var.vpc_cidr
  availability_zones   = var.availability_zones
  public_subnet_cidrs  = var.public_subnet_cidrs
  private_subnet_cidrs = var.private_subnet_cidrs
  tags                 = local.common_tags
}

# Security Group Module
module "security_group" {
  source = "../../modules/security-group"

  sg_name     = "${local.name_prefix}-sg"
  description = "Security group for ${var.project_name} application"
  vpc_id      = module.vpc.vpc_id

  ingress_rules = [
    {
      description = "HTTP"
      from_port   = 80
      to_port     = 80
      protocol    = "tcp"
      cidr_ipv4   = "0.0.0.0/0"
    },
    {
      description = "HTTPS"
      from_port   = 443
      to_port     = 443
      protocol    = "tcp"
      cidr_ipv4   = "0.0.0.0/0"
    },
    {
      description = "Application Port"
      from_port   = 8080
      to_port     = 8080
      protocol    = "tcp"
      cidr_ipv4   = "0.0.0.0/0"
    },
    {
      description = "SSH"
      from_port   = 22
      to_port     = 22
      protocol    = "tcp"
      cidr_ipv4   = "0.0.0.0/0"
    }
  ]

  egress_rules = [
    {
      description = "All outbound"
      protocol    = "-1"
      cidr_ipv4   = "0.0.0.0/0"
    }
  ]

  tags = local.common_tags
}

# DynamoDB Tables
module "dynamodb_movie" {
  source = "../../modules/dynamodb"

  table_name                    = local.dynamodb_movie_table
  billing_mode                  = var.dynamodb_billing_mode
  hash_key                      = "id"
  attributes                    = [{ name = "id", type = "S" }]
  read_capacity                 = var.dynamodb_read_capacity
  write_capacity                = var.dynamodb_write_capacity
  enable_point_in_time_recovery = true
  enable_encryption             = true
  tags                          = local.common_tags
}

module "dynamodb_person" {
  source = "../../modules/dynamodb"

  table_name                    = local.dynamodb_person_table
  billing_mode                  = var.dynamodb_billing_mode
  hash_key                      = "id"
  attributes                    = [{ name = "id", type = "S" }]
  read_capacity                 = var.dynamodb_read_capacity
  write_capacity                = var.dynamodb_write_capacity
  enable_point_in_time_recovery = true
  enable_encryption             = true
  tags                          = local.common_tags
}

module "dynamodb_attribute" {
  source = "../../modules/dynamodb"

  table_name                    = local.dynamodb_attribute_table
  billing_mode                  = var.dynamodb_billing_mode
  hash_key                      = "id"
  attributes                    = [{ name = "id", type = "S" }]
  read_capacity                 = var.dynamodb_read_capacity
  write_capacity                = var.dynamodb_write_capacity
  enable_point_in_time_recovery = true
  enable_encryption             = true
  tags                          = local.common_tags
}

module "dynamodb_encoder" {
  source = "../../modules/dynamodb"

  table_name   = local.dynamodb_encoder_table
  billing_mode = var.dynamodb_billing_mode
  hash_key     = "id"
  attributes = [
    { name = "id", type = "S" },
    { name = "contentId", type = "S" },
  ]
  global_secondary_indexes = [
    {
      name            = "contentId-index"
      hash_key        = "contentId"
      projection_type = "ALL"
    },
  ]
  read_capacity                 = var.dynamodb_read_capacity
  write_capacity                = var.dynamodb_write_capacity
  enable_point_in_time_recovery = true
  enable_encryption             = true
  tags                          = local.common_tags
}

# S3 Bucket
module "s3" {
  source = "../../modules/s3"

  bucket_name         = local.s3_bucket
  enable_versioning   = var.enable_versioning
  enable_encryption   = var.enable_encryption
  block_public_access = false
  enable_public_read  = true

  cors_rules = [
    {
      allowed_headers = ["*"]
      allowed_methods = ["GET", "PUT", "POST", "DELETE", "HEAD"]
      allowed_origins = [
        "http://localhost:3000",
        "http://localhost:8080",
        "http://localhost:8443",
        "https://localhost:8443",
        "http://127.0.0.1:3000",
        "http://127.0.0.1:8080",
        "http://127.0.0.1:8443",
        "https://127.0.0.1:8443",
        "https://api.xanderbilla.com",
        "https://grafana.xanderbilla.com",
        "https://storage.xanderbilla.com"
      ]
      expose_headers  = ["ETag"]
      max_age_seconds = 3000
    }
  ]

  lifecycle_rules = [
    {
      id     = "transition-old-versions"
      status = "Enabled"
      noncurrent_version_transitions = [
        {
          noncurrent_days = 30
          storage_class   = "STANDARD_IA"
        },
        {
          noncurrent_days = 90
          storage_class   = "GLACIER"
        }
      ]
      noncurrent_version_expiration = {
        noncurrent_days = 365
      }
    },
    {
      id     = "delete-incomplete-uploads"
      status = "Enabled"
      abort_incomplete_multipart_upload = {
        days_after_initiation = 7
      }
    },
    {
      id         = "expire-log-chunks"
      status     = "Enabled"
      filter     = { prefix = "logs/" }
      expiration = { days = 30 }
      abort_incomplete_multipart_upload = {
        days_after_initiation = 1
      }
    },
    {
      id         = "expire-traces"
      status     = "Enabled"
      filter     = { prefix = "traces/" }
      expiration = { days = 14 }
      abort_incomplete_multipart_upload = {
        days_after_initiation = 1
      }
    }
  ]

  tags = local.common_tags
}

# IAM Role
module "iam" {
  source = "../../modules/iam"

  role_name    = "${local.name_prefix}-ec2-role"
  project_name = var.project_name
  aws_region   = var.aws_region

  dynamodb_table_arns = [
    module.dynamodb_movie.table_arn,
    module.dynamodb_person.table_arn,
    module.dynamodb_attribute.table_arn,
    module.dynamodb_encoder.table_arn
  ]

  s3_bucket_arns = [
    module.s3.bucket_arn,
  ]

  tags = local.common_tags
}

# EC2 Instance
module "ec2" {
  source = "../../modules/ec2"

  instance_name        = "${local.name_prefix}-instance"
  instance_type        = var.instance_type
  ami_id               = var.ami_id
  subnet_id            = module.vpc.public_subnet_ids[0]
  security_group_ids   = [module.security_group.security_group_id]
  iam_instance_profile = module.iam.instance_profile_name
  key_name             = var.key_name
  create_eip           = true

  user_data = base64gzip(templatefile("${path.module}/user-data.sh", {
    project_name             = var.project_name
    environment              = var.environment
    aws_region               = var.aws_region
    dynamodb_movie_table     = local.dynamodb_movie_table
    dynamodb_person_table    = local.dynamodb_person_table
    dynamodb_attribute_table = local.dynamodb_attribute_table
    dynamodb_encoder_table   = local.dynamodb_encoder_table
    s3_bucket                = local.s3_bucket
    prometheus_device        = "/dev/xvdb"
    repo_url                 = var.repo_url
    repo_branch              = var.repo_branch
    image_name               = var.image_name
    grafana_admin_user       = var.grafana_admin_user
    grafana_admin_password   = var.grafana_admin_password
    grafana_domain_name      = var.grafana_domain_name
    storage_domain_name      = var.storage_domain_name
    domain_name              = var.domain_name
    admin_email              = var.admin_email
  }))

  tags = local.common_tags
}

# Resolve the AZ of the primary public subnet so the EBS volume is co-located
data "aws_subnet" "primary" {
  id = module.vpc.public_subnet_ids[0]
}

# EBS Volume for Prometheus persistent data storage
resource "aws_ebs_volume" "prometheus" {
  availability_zone = data.aws_subnet.primary.availability_zone
  size              = 20
  type              = "gp3"
  encrypted         = true

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-prometheus-data"
  })
}

resource "aws_volume_attachment" "prometheus" {
  device_name = "/dev/xvdb"
  volume_id   = aws_ebs_volume.prometheus.id
  instance_id = module.ec2.instance_id
}

# Route53 A record — auto-updated to the EC2 EIP on every deploy
resource "aws_route53_record" "api" {
  zone_id = var.route53_zone_id
  name    = var.domain_name
  type    = "A"
  ttl     = 60
  records = [module.ec2.instance_public_ip]
}

resource "aws_route53_record" "grafana" {
  zone_id = var.route53_zone_id
  name    = var.grafana_domain_name
  type    = "A"
  ttl     = 60
  records = [module.ec2.instance_public_ip]
}
