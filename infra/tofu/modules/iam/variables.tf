variable "role_name" {
  description = "Name of the IAM role"
  type        = string
}

variable "project_name" {
  description = "Project name"
  type        = string
}

variable "aws_region" {
  description = "AWS region"
  type        = string
}

variable "dynamodb_table_arns" {
  description = "List of DynamoDB table ARNs"
  type        = list(string)
  default     = []
}

variable "s3_bucket_arns" {
  description = "List of S3 bucket ARNs"
  type        = list(string)
  default     = []
}

variable "opensearch_domain_arns" {
  description = "List of OpenSearch domain ARNs the role may call via es:ESHttp*. Empty list skips the policy."
  type        = list(string)
  default     = []
}

variable "ecr_repository_arns" {
  description = "ECR repository ARNs the role may pull from. Empty list grants pull on any repository in the account (legacy behavior)."
  type        = list(string)
  default     = []
}

variable "ssm_parameter_path" {
  description = "SSM Parameter Store path prefix the role may read (e.g. /bi8s/dev). Empty string skips the SSM parameter policy."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Tags to apply to resources"
  type        = map(string)
  default     = {}
}
