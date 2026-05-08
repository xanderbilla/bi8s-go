# AWS Budgets module
#
# Creates a monthly cost budget for the account scoped to the project's tag
# (Project = var.project_name) and notifies the configured email addresses
# when forecasted or actual spend crosses the configured thresholds. Budgets
# themselves are free of charge.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

resource "aws_budgets_budget" "monthly" {
  name              = "${var.project_name}-${var.environment}-monthly"
  budget_type       = "COST"
  limit_amount      = tostring(var.monthly_limit_usd)
  limit_unit        = "USD"
  time_unit         = "MONTHLY"
  time_period_start = var.time_period_start

  # AWS Budgets cost-filter tag values use the format `user:<TagKey>$<TagValue>`.
  # The literal `$` between key and value is escaped as `$$` so Terraform does
  # not interpret it as interpolation.
  cost_filter {
    name = "TagKeyValue"
    values = [
      "user:Project$$${var.project_name}",
    ]
  }

  dynamic "notification" {
    for_each = var.actual_thresholds_pct
    content {
      comparison_operator        = "GREATER_THAN"
      threshold                  = notification.value
      threshold_type             = "PERCENTAGE"
      notification_type          = "ACTUAL"
      subscriber_email_addresses = var.notification_emails
    }
  }

  dynamic "notification" {
    for_each = var.forecasted_thresholds_pct
    content {
      comparison_operator        = "GREATER_THAN"
      threshold                  = notification.value
      threshold_type             = "PERCENTAGE"
      notification_type          = "FORECASTED"
      subscriber_email_addresses = var.notification_emails
    }
  }
}
