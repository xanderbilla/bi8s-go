variable "project_name" {
  description = "Project name"
  type        = string
  default     = "bi8s"
}

variable "environment" {
  description = "Environment (dev, prod, staging)"
  type        = string
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}
