variable "project_name" {
  description = "Project name; must match the Project tag applied by other modules."
  type        = string
}

variable "environment" {
  description = "Environment name (dev, prod, ...)."
  type        = string
}

variable "monthly_limit_usd" {
  description = "Monthly budget limit in USD."
  type        = number
}

variable "notification_emails" {
  description = "List of email addresses to notify when thresholds are crossed."
  type        = list(string)
  validation {
    condition     = length(var.notification_emails) > 0
    error_message = "notification_emails must contain at least one address."
  }
}

variable "actual_thresholds_pct" {
  description = "Percent-of-limit thresholds that trigger ACTUAL spend alerts."
  type        = list(number)
  default     = [80, 100]
}

variable "forecasted_thresholds_pct" {
  description = "Percent-of-limit thresholds that trigger FORECASTED spend alerts."
  type        = list(number)
  default     = [100]
}

variable "time_period_start" {
  description = "Budget start date in YYYY-MM-DD_HH:MM format. Defaults to a stable historical date so plan output is deterministic."
  type        = string
  default     = "2024-01-01_00:00"
}
