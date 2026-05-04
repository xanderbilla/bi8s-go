# GitHub Actions OIDC provider + per-environment deploy role.
#
# The OIDC provider is account-wide and may already exist (created by another
# project / bootstrap stack). Set var.create_oidc_provider = false to reuse the
# existing provider rather than re-creating it.
#
# The role's trust policy restricts assumption to:
#   1. The configured repository (var.github_owner/var.github_repo).
#   2. Either the configured ref (`refs/heads/<branch>`) or GitHub Environment
#      (`environment:<name>`). At least one must be configured.

locals {
  oidc_provider_arn = (
    var.create_oidc_provider
    ? aws_iam_openid_connect_provider.github[0].arn
    : "arn:aws:iam::${data.aws_caller_identity.current.account_id}:oidc-provider/token.actions.githubusercontent.com"
  )

  repo_subject_prefix = "repo:${var.github_owner}/${var.github_repo}"

  ref_subjects         = [for b in var.allowed_branches : "${local.repo_subject_prefix}:ref:refs/heads/${b}"]
  environment_subjects = [for e in var.allowed_environments : "${local.repo_subject_prefix}:environment:${e}"]
  pull_request_subject = var.allow_pull_requests ? ["${local.repo_subject_prefix}:pull_request"] : []

  all_subjects = concat(local.ref_subjects, local.environment_subjects, local.pull_request_subject)
}

data "aws_caller_identity" "current" {}

resource "aws_iam_openid_connect_provider" "github" {
  count = var.create_oidc_provider ? 1 : 0

  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]
  # GitHub's published thumbprint. Modern AWS no longer requires verifying it
  # (uses the system trust store), but the API still requires a non-empty list.
  thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1"]

  tags = var.tags
}

data "aws_iam_policy_document" "trust" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [local.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      values   = local.all_subjects
    }
  }
}

resource "aws_iam_role" "deploy" {
  name                 = var.role_name
  assume_role_policy   = data.aws_iam_policy_document.trust.json
  max_session_duration = var.max_session_duration

  tags = var.tags
}

# Inline scoped policy. Wide enough to manage the full app stack (VPC, EC2,
# IAM, DynamoDB, S3, ECR, Route53, CloudFront, ACM, SSM, CloudWatch Logs) but
# tighter than AdministratorAccess. Tighten further as appropriate.
data "aws_iam_policy_document" "deploy" {
  statement {
    sid    = "InfraReadWrite"
    effect = "Allow"
    actions = [
      "ec2:*",
      "vpc:*",
      "elasticloadbalancing:*",
      "autoscaling:*",
      "dynamodb:*",
      "s3:*",
      "ecr:*",
      "ecr-public:*",
      "route53:*",
      "cloudfront:*",
      "acm:*",
      "ssm:*",
      "logs:*",
      "cloudwatch:*",
      "events:*",
      "kms:Describe*",
      "kms:List*",
      "kms:Get*",
      "kms:Encrypt",
      "kms:Decrypt",
      "kms:ReEncrypt*",
      "kms:GenerateDataKey*",
      "sts:GetCallerIdentity",
    ]
    resources = ["*"]
  }

  statement {
    sid    = "IamForServiceRoles"
    effect = "Allow"
    actions = [
      "iam:GetRole",
      "iam:GetRolePolicy",
      "iam:ListRolePolicies",
      "iam:ListAttachedRolePolicies",
      "iam:GetPolicy",
      "iam:GetPolicyVersion",
      "iam:ListPolicyVersions",
      "iam:GetInstanceProfile",
      "iam:CreateRole",
      "iam:DeleteRole",
      "iam:UpdateRole",
      "iam:UpdateAssumeRolePolicy",
      "iam:PutRolePolicy",
      "iam:DeleteRolePolicy",
      "iam:AttachRolePolicy",
      "iam:DetachRolePolicy",
      "iam:CreatePolicy",
      "iam:DeletePolicy",
      "iam:CreatePolicyVersion",
      "iam:DeletePolicyVersion",
      "iam:CreateInstanceProfile",
      "iam:DeleteInstanceProfile",
      "iam:AddRoleToInstanceProfile",
      "iam:RemoveRoleFromInstanceProfile",
      "iam:PassRole",
      "iam:TagRole",
      "iam:UntagRole",
      "iam:TagPolicy",
      "iam:UntagPolicy",
      "iam:TagInstanceProfile",
      "iam:UntagInstanceProfile",
      "iam:ListRoles",
      "iam:ListPolicies",
      "iam:ListInstanceProfiles",
      "iam:ListInstanceProfilesForRole",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "deploy" {
  name   = "${var.role_name}-policy"
  role   = aws_iam_role.deploy.id
  policy = data.aws_iam_policy_document.deploy.json
}
