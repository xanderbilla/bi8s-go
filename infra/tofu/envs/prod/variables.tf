variable "project_name" {
  description = "Project name"
  type        = string
  default     = "bi8s"
}

variable "environment" {
  description = "Environment"
  type        = string
  default     = "prod"
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "vpc_cidr" {
  description = "VPC CIDR block"
  type        = string
  default     = "10.1.0.0/16"
}

variable "availability_zones" {
  description = "Availability zones"
  type        = list(string)
  default     = ["us-east-1a", "us-east-1b"]
}

variable "public_subnet_cidrs" {
  description = "Public subnet CIDRs"
  type        = list(string)
  default     = ["10.1.1.0/24", "10.1.2.0/24"]
}

variable "private_subnet_cidrs" {
  description = "Private subnet CIDRs"
  type        = list(string)
  default     = ["10.1.10.0/24", "10.1.11.0/24"]
}

variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "t2.medium"
}

variable "ami_id" {
  description = "AMI ID"
  type        = string
  default     = "ami-0ec10929233384c7f"
}

variable "key_name" {
  description = "SSH key name"
  type        = string
  default     = "go-server"
}

variable "dynamodb_billing_mode" {
  description = "DynamoDB billing mode"
  type        = string
  default     = "PAY_PER_REQUEST"
}

variable "dynamodb_read_capacity" {
  description = "DynamoDB read capacity"
  type        = number
  default     = 5
}

variable "dynamodb_write_capacity" {
  description = "DynamoDB write capacity"
  type        = number
  default     = 5
}

variable "enable_versioning" {
  description = "Enable S3 versioning"
  type        = bool
  default     = true
}

variable "enable_encryption" {
  description = "Enable S3 encryption"
  type        = bool
  default     = true
}

variable "tags" {
  description = "Additional tags"
  type        = map(string)
  default     = {}
}

# ---------------------------------------------------------------------------
# Application / deployment variables (mirroring dev for parity)
# ---------------------------------------------------------------------------

variable "repo_url" {
  description = "Git repository to clone on the EC2 instance for compose/observability assets"
  type        = string
  default     = "https://github.com/xanderbilla/bi8s-go.git"
}

variable "repo_branch" {
  description = "Git branch to deploy"
  type        = string
  default     = "prod"
}

variable "route53_zone_id" {
  description = "Route53 hosted zone ID. Only used when enable_public_dns=true."
  type        = string
  default     = ""
}

variable "domain_name" {
  description = "Public domain for the API. Empty + enable_public_dns=false means access via raw IP only."
  type        = string
  default     = ""
}

variable "grafana_admin_user" {
  description = "Grafana admin username"
  type        = string
  default     = "admin"
}

variable "grafana_admin_password" {
  description = "Grafana admin password (must be supplied via terraform.tfvars or TF_VAR_grafana_admin_password)"
  type        = string
  sensitive   = true
}

variable "grafana_domain_name" {
  description = "Grafana subdomain (empty disables the dedicated Grafana nginx vhost)"
  type        = string
  default     = ""
}

variable "storage_domain_name" {
  description = "CDN/storage domain (empty falls back to public IP for STORAGE_BASE_URL)"
  type        = string
  default     = ""
}

variable "admin_email" {
  description = "Admin email used for Let's Encrypt registration"
  type        = string
  default     = ""
}

variable "enable_public_dns" {
  description = "When true, request Let's Encrypt certs and create Route53 records. Default false for personal-project prod (raw-IP access acceptable)."
  type        = bool
  default     = false
}
