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

# ---------------------------------------------------------------------------
# GitHub Actions OIDC (replaces long-lived AWS access keys)
# ---------------------------------------------------------------------------

variable "github_owner" {
  description = "GitHub organization or user that owns the repo"
  type        = string
  default     = "xanderbilla"
}

variable "github_repo" {
  description = "GitHub repository name"
  type        = string
  default     = "bi8s-go"
}

variable "create_oidc_provider" {
  description = "Create the account-wide GitHub OIDC provider. Set true the first time (e.g. dev bootstrap), false thereafter."
  type        = bool
  default     = true
}

variable "allow_pr_plan" {
  description = "When true, the role may also be assumed from pull_request workflows (use only for plan-only/read-only flows). Recommended only for dev."
  type        = bool
  default     = false
}
