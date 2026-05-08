resource "aws_security_group" "this" {
  name        = var.security_group_name
  description = "Security group for OpenSearch domain"
  vpc_id      = var.vpc_id

  ingress {
    description     = "HTTPS from allowed security groups"
    from_port       = 443
    to_port         = 443
    protocol        = "tcp"
    security_groups = var.allowed_security_group_ids
  }

  egress {
    description = "All outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = var.tags
}

resource "aws_opensearch_domain" "this" {
  domain_name    = var.domain_name
  engine_version = var.engine_version

  cluster_config {
    instance_type            = var.instance_type
    instance_count           = var.instance_count
    zone_awareness_enabled   = var.zone_awareness_enabled
    dedicated_master_enabled = false
  }

  ebs_options {
    ebs_enabled = true
    volume_type = var.volume_type
    volume_size = var.volume_size
  }

  vpc_options {
    subnet_ids         = var.subnet_ids
    security_group_ids = [aws_security_group.this.id]
  }

  encrypt_at_rest {
    enabled = true
  }

  node_to_node_encryption {
    enabled = true
  }

  domain_endpoint_options {
    enforce_https       = true
    tls_security_policy = "Policy-Min-TLS-1-2-2019-07"
  }

  advanced_security_options {
    enabled                        = false
    internal_user_database_enabled = false
  }

  access_policies = data.aws_iam_policy_document.domain_access.json

  tags = var.tags
}

data "aws_iam_policy_document" "domain_access" {
  statement {
    sid     = "AllowVpcAccess"
    effect  = "Allow"
    actions = ["es:ESHttp*"]

    principals {
      type        = "AWS"
      identifiers = ["*"]
    }

    resources = ["arn:aws:es:${var.aws_region}:${var.account_id}:domain/${var.domain_name}/*"]
  }
}
