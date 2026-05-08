// CloudWatch log groups for the EC2-hosted app + nginx tail-shippers.
// Retention defaults to 30d in prod (see var.log_retention_days) so incident
// forensics windows survive routine log rotation.

resource "aws_cloudwatch_log_group" "app" {
  name              = "/aws/ec2/${local.name_prefix}/app"
  retention_in_days = var.log_retention_days

  tags = merge(local.common_tags, {
    Name      = "${local.name_prefix}-app-logs"
    Component = "app"
  })
}

resource "aws_cloudwatch_log_group" "nginx" {
  name              = "/aws/ec2/${local.name_prefix}/nginx"
  retention_in_days = var.log_retention_days

  tags = merge(local.common_tags, {
    Name      = "${local.name_prefix}-nginx-logs"
    Component = "nginx"
  })
}
