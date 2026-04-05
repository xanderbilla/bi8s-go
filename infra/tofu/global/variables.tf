# Global Variables - Shared across all environments

variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
  default     = "bi8s"
}

variable "aws_region" {
  description = "AWS region for resources"
  type        = string
  default     = "us-east-1"
}

variable "common_tags" {
  description = "Common tags to apply to all resources"
  type        = map(string)
  default = {
    Project   = "bi8s"
    ManagedBy = "OpenTofu"
  }
}
