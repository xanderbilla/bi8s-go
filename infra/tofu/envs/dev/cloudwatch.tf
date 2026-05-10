// CloudWatch log groups for the EC2-hosted app + nginx tail-shippers.
// The CloudWatch agent installed via infra/scripts/install-cloudwatch-agent.sh
// pushes container logs to these groups. Defining them here ensures retention
// is set declaratively (default 7d in dev) and the groups survive instance
// recreation.

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
