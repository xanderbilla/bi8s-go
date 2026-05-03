variable "project_name" {
  description = "Project name"
  type        = string
  default     = "bi8s"
}

variable "environment" {
  description = "Environment"
  type        = string
  default     = "dev"
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "vpc_cidr" {
  description = "VPC CIDR block"
  type        = string
  default     = "10.0.0.0/16"
}

variable "availability_zones" {
  description = "Availability zones"
  type        = list(string)
  default     = ["us-east-1a", "us-east-1b"]
}

variable "public_subnet_cidrs" {
  description = "Public subnet CIDRs"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "private_subnet_cidrs" {
  description = "Private subnet CIDRs"
  type        = list(string)
  default     = ["10.0.10.0/24", "10.0.11.0/24"]
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

variable "repo_url" {
  description = "Public Git repository URL cloned by EC2 user-data for compose + observability configs"
  type        = string
  default     = "https://github.com/xanderbilla/bi8s-go.git"
}

variable "repo_branch" {
  description = "Git branch/tag to check out on the EC2 instance"
  type        = string
  default     = "dev"
}

variable "route53_zone_id" {
  description = "Route53 hosted zone ID for auto-updating the DNS A record on deploy"
  type        = string
  default     = "Z001129137OC90Z9NK3SB"
}

variable "domain_name" {
  description = "Domain name (A record) to point at the EC2 EIP"
  type        = string
  default     = "api.xanderbilla.com"
}

variable "grafana_admin_user" {
  description = "Admin username for the Grafana UI on the EC2 instance"
  type        = string
  default     = "admin"
}

variable "grafana_admin_password" {
  description = "Admin password for the Grafana UI on the EC2 instance"
  type        = string
  sensitive   = true
  default     = "admin"
}

variable "grafana_domain_name" {
  description = "Domain name for Grafana subdomain"
  type        = string
  default     = "grafana.xanderbilla.com"
}

variable "storage_domain_name" {
  description = "Domain name for storage CDN"
  type        = string
  default     = "storage.xanderbilla.com"
}

variable "admin_email" {
  description = "Admin email for Let's Encrypt certificate registration"
  type        = string
  default     = "admin@xanderbilla.com"
}
