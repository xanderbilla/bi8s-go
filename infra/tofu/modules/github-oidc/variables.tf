variable "github_owner" {
  description = "GitHub organization or user that owns the repository (e.g. xanderbilla)"
  type        = string
}

variable "github_repo" {
  description = "GitHub repository name (e.g. bi8s-go)"
  type        = string
}

variable "role_name" {
  description = "IAM role name to create for GitHub Actions"
  type        = string
}

variable "allowed_branches" {
  description = "List of git branches whose workflows may assume this role (e.g. ['dev'] or ['prod'])."
  type        = list(string)
  default     = []
}

variable "allowed_environments" {
  description = "List of GitHub Environment names whose workflows may assume this role (e.g. ['prod']). Use to require manual reviewer approval before assumption."
  type        = list(string)
  default     = []
}

variable "allow_pull_requests" {
  description = "When true, also allow assumption from pull_request workflows in this repo. Use sparingly — restricts to read-only/plan-only roles."
  type        = bool
  default     = false
}

variable "create_oidc_provider" {
  description = "Create the GitHub OIDC provider in this account. Set false if it already exists."
  type        = bool
  default     = true
}

variable "max_session_duration" {
  description = "Maximum session duration in seconds for assumed sessions"
  type        = number
  default     = 3600
}

variable "tags" {
  description = "Tags applied to created IAM resources"
  type        = map(string)
  default     = {}
}
