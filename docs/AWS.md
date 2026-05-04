# AWS

This document describes every AWS resource `bi8s-go` touches and the IAM
permissions it requires. All resources are provisioned by the Tofu
modules under `infra/tofu/modules/`.

## Account model

A single AWS account per environment (`dev`, `prod`). State is stored
remotely in S3 with DynamoDB locking — see `infra/tofu/bootstrap/`.

## Region

All resources live in **a single region** (default `ap-south-1` for
`dev`). Set via the `aws_region` Tofu variable and the `AWS_REGION` env
var on the running app.

## Resources

### EC2

| Resource                       | Purpose                                               |
| ------------------------------ | ----------------------------------------------------- |
| `aws_instance.api`             | Runs Docker; hosts the API container + NGINX.         |
| `aws_eip`                      | Stable public IP.                                     |
| `aws_security_group.api`       | Allows 80/443 from the world, 22 from a bastion CIDR. |
| `aws_iam_instance_profile.api` | Attaches the runtime IAM role (below).                |

### DynamoDB

Four tables, all defined in `infra/tofu/modules/dynamodb/`:

| Table (dev)                 | Primary key       | GSIs                            |
| --------------------------- | ----------------- | ------------------------------- |
| `bi8s-content-table-dev`    | `contentId` (S)   | —                               |
| `bi8s-person-table-dev`     | `personId` (S)    | —                               |
| `bi8s-attributes-table-dev` | `attributeId` (S) | `name-index` (`name`)           |
| `bi8s-video-table-dev`      | `jobId` (S)       | `contentId-index` (`contentId`) |

`dev` uses provisioned capacity (low minimums + autoscaling); `prod`
uses on-demand. Point-in-time recovery is on for both.

### S3

| Bucket (dev)       | Purpose                                                                                      |
| ------------------ | -------------------------------------------------------------------------------------------- |
| `bi8s-storage-dev` | Single bucket: uploaded sources, HLS output, Loki + Tempo blob backends (separate prefixes). |

Settings: server-side encryption (SSE-S3), versioning enabled, block
public access on, lifecycle rules for incomplete multipart uploads
(`abort after 7d`) and Loki/Tempo block expiry (`30d` in dev).

### SQS _(optional)_

When `ENCODER_QUEUE_URL` is set, encoder jobs are published to SQS
instead of being processed in-process. Provisioned ad-hoc; not in the
default Tofu config.

### CloudWatch

- Log group `/bi8s/api/<env>` — populated by the CloudWatch Agent on
  EC2 (config in `infra/scripts/install-cloudwatch-agent.sh`).
- Alarms for ALB 5xx rate, EC2 CPU, DynamoDB throttles.

## IAM

### Instance role (runtime)

Minimum policy attached to the EC2 instance profile:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "Dynamo",
      "Effect": "Allow",
      "Action": [
        "dynamodb:GetItem",
        "dynamodb:PutItem",
        "dynamodb:UpdateItem",
        "dynamodb:DeleteItem",
        "dynamodb:Query",
        "dynamodb:Scan",
        "dynamodb:BatchGetItem",
        "dynamodb:BatchWriteItem",
        "dynamodb:DescribeTable"
      ],
      "Resource": [
        "arn:aws:dynamodb:*:*:table/bi8s-*-table-*",
        "arn:aws:dynamodb:*:*:table/bi8s-*-table-*/index/*"
      ]
    },
    {
      "Sid": "S3",
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:AbortMultipartUpload",
        "s3:ListMultipartUploadParts",
        "s3:ListBucket",
        "s3:GetBucketLocation"
      ],
      "Resource": [
        "arn:aws:s3:::bi8s-storage-*",
        "arn:aws:s3:::bi8s-storage-*/*"
      ]
    },
    {
      "Sid": "Logs",
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogStream",
        "logs:PutLogEvents",
        "logs:DescribeLogStreams"
      ],
      "Resource": "arn:aws:logs:*:*:log-group:/bi8s/api/*:*"
    }
  ]
}
```

### CI role (GitHub OIDC)

`infra/tofu/modules/github-oidc/` provisions an OIDC provider and a role
trusted only by the `xanderbilla/bi8s-go` repository. The role grants
the permissions required to apply Tofu changes (subset of admin: VPC,
EC2, IAM, S3, DynamoDB, ECR/GHCR pull). No long-lived access keys are
issued or stored.

## Cost notes

- DynamoDB on-demand in prod scales to zero billing during quiet hours.
- S3 lifecycle rules trim old HLS renditions and Loki/Tempo blocks.
- A single `t4g.small` EC2 instance handles dev. Prod sizing depends on
  catalog/encode volume.
- OTel collector + Tempo + Loki run _on the same EC2_ in dev (no
  managed Grafana Cloud). For prod, swap exporters to Grafana Cloud or
  AMP/Managed Grafana to externalize storage.
