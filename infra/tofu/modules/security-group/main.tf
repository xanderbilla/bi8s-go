# Security Group Module
resource "aws_security_group" "this" {
  name        = var.sg_name
  description = var.description
  vpc_id      = var.vpc_id

  # Revoke all rules before deleting the SG. Without this, destroy can fail
  # with "DependencyViolation" when another resource still references this SG
  # (e.g. an ENI from a recently-terminated EC2 not yet fully released by AWS).
  revoke_rules_on_delete = true

  tags = merge(
    var.tags,
    {
      Name = var.sg_name
    }
  )
}

# Ingress Rules
resource "aws_vpc_security_group_ingress_rule" "this" {
  for_each = { for idx, rule in var.ingress_rules : idx => rule }

  security_group_id            = aws_security_group.this.id
  description                  = each.value.description
  from_port                    = each.value.from_port
  to_port                      = each.value.to_port
  ip_protocol                  = each.value.protocol
  cidr_ipv4                    = lookup(each.value, "cidr_ipv4", null)
  referenced_security_group_id = lookup(each.value, "source_security_group_id", null)
}

# Egress Rules
resource "aws_vpc_security_group_egress_rule" "this" {
  for_each = { for idx, rule in var.egress_rules : idx => rule }

  security_group_id = aws_security_group.this.id
  description       = each.value.description
  from_port         = lookup(each.value, "from_port", null)
  to_port           = lookup(each.value, "to_port", null)
  ip_protocol       = each.value.protocol
  cidr_ipv4         = lookup(each.value, "cidr_ipv4", null)
}
