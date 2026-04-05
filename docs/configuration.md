# Configuration Guide

This document explains how to configure the project for your deployment.

## Project Name Configuration

The project name is the most important configuration as it's used to name all AWS resources.

### Where to Configure

#### 1. GitHub Actions Workflow (Required)

File: `.github/workflows/infra-deploy.yml`

```yaml
env:
  TF_VERSION: "1.6.0"
  AWS_REGION: ${{ secrets.AWS_REGION }}
  PROJECT_NAME: "bi8s" # Change this to your project name
```

Change `"bi8s"` to your project name (lowercase, alphanumeric, hyphens allowed).

#### 2. Terraform Variables (Optional - has default)

File: `infra/tofu/envs/dev/variables.tf` and `infra/tofu/envs/prod/variables.tf`

```hcl
variable "project_name" {
  description = "Project name"
  type        = string
  default     = "bi8s"  # Change this to match GitHub workflow
}
```

**Note:** If you don't change this, the GitHub workflow value will override it anyway.

### Resource Naming Convention

The project name is used to create resource names following this pattern:

```
{project-name}-{resource}-{environment}
```

Examples with project name "myapp":

**DynamoDB Tables:**

- `myapp-content-table-dev`
- `myapp-person-table-dev`
- `myapp-attributes-table-dev`
- `myapp-video-table-dev`

**S3 Bucket:**

- `myapp-storage-dev`

**EC2 Instance:**

- `myapp-dev-instance`

**Security Group:**

- `myapp-dev-sg`

**VPC:**

- `myapp-dev-vpc`

**IAM Role:**

- `myapp-dev-ec2-role`

**Terraform State (S3):**

- `myapp-terraform-state-dev`

**Terraform Locks (DynamoDB):**

- `myapp-terraform-locks-dev`

### Rules for Project Name

- Use lowercase letters only
- Use alphanumeric characters and hyphens
- Start with a letter
- Keep it short (3-20 characters recommended)
- Avoid special characters except hyphens
- Must be unique in your AWS account

**Good examples:**

- `myapp`
- `my-project`
- `acme-api`
- `content-platform`

**Bad examples:**

- `MyApp` (uppercase)
- `my_app` (underscore)
- `my.app` (dot)
- `123app` (starts with number)

## Environment Configuration

Environments are automatically detected from the git branch:

- `dev` branch → dev environment
- `main` branch → prod environment

### Environment-Specific Variables

Each environment has its own variables file:

- `infra/tofu/envs/dev/variables.tf`
- `infra/tofu/envs/prod/variables.tf`

You can customize per environment:

```hcl
# Dev environment - smaller/cheaper resources
variable "instance_type" {
  default = "t3.micro"
}

variable "dynamodb_billing_mode" {
  default = "PAY_PER_REQUEST"
}
```

```hcl
# Prod environment - larger/production resources
variable "instance_type" {
  default = "t3.small"
}

variable "dynamodb_billing_mode" {
  default = "PROVISIONED"
}
```

## AWS Configuration

### Required Secrets

Configure these in GitHub repository settings (Settings → Secrets and variables → Actions):

```
AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
AWS_REGION=us-east-1
```

### IAM Permissions Required

The AWS credentials need these permissions:

- EC2: Full access (create instances, security groups, VPC)
- DynamoDB: Full access (create tables)
- S3: Full access (create buckets)
- IAM: Create roles and policies
- CloudWatch: Create log groups

Recommended: Use a dedicated IAM user for CI/CD with these permissions.

### SSH Key Configuration

For EC2 access, configure SSH key name in Terraform variables:

```hcl
variable "key_name" {
  description = "SSH key name"
  type        = string
  default     = "my-ec2-key"  # Your EC2 key pair name
}
```

The key pair must exist in AWS before deployment.

## Docker Configuration

### Docker Hub Credentials

Configure these in GitHub repository settings:

```
DOCKER_REGISTRY=docker.io
DOCKER_USERNAME=your-dockerhub-username
DOCKER_PASSWORD=your-dockerhub-token
```

**Note:** Use a Docker Hub access token, not your password.

### Docker Image Name

Update in `.github/workflows/docker-publish.yml`:

```yaml
env:
  IMAGE_NAME: docker.io/your-username/your-project
```

Also update in `infra/tofu/envs/dev/user-data.sh`:

```bash
IMAGE_NAME=your-username/your-project:latest
```

## Application Configuration

### Environment Variables on EC2

After EC2 is created, configure application secrets in `/opt/bi8s/compose/.env`:

```bash
# Auto-configured by Terraform (don't change)
PROJECT_NAME=myapp
IMAGE_NAME=username/myapp:latest
APP_ENV=dev
AWS_REGION=us-east-1
DYNAMODB_MOVIE_TABLE=myapp-content-table-dev
DYNAMODB_PERSON_TABLE=myapp-person-table-dev
DYNAMODB_ATTRIBUTE_TABLE=myapp-attributes-table-dev
DYNAMODB_ENCODER_TABLE=myapp-video-table-dev
S3_BUCKET=myapp-storage-dev
CORS_ALLOWED_ORIGINS=*
CORS_ALLOW_PRIVATE_NETWORK=true
PUBLIC_IP=auto-detected

# Add your custom secrets below
JWT_SECRET=your-jwt-secret-key
API_KEY=your-api-key
# Add more as needed
```

## CORS Configuration

Configure allowed origins in Terraform variables:

```hcl
# Allow all origins (development)
CORS_ALLOWED_ORIGINS=*

# Allow specific origins (production)
CORS_ALLOWED_ORIGINS=https://yourdomain.com,https://app.yourdomain.com
```

## SSL Configuration

### Self-Signed Certificate (Default)

Automatically generated on EC2 boot. No configuration needed.

### Let's Encrypt (Production)

Requires a domain name. Configure after deployment:

1. Point domain to EC2 IP
2. Run: `./infra/scripts/setup-ssl-letsencrypt.sh <EC2_IP> api.yourdomain.com`

## Resource Sizing

### Development Environment

```hcl
# infra/tofu/envs/dev/variables.tf
variable "instance_type" {
  default = "t3.micro"  # 1 vCPU, 1 GB RAM
}

variable "dynamodb_billing_mode" {
  default = "PAY_PER_REQUEST"  # Pay only for what you use
}
```

### Production Environment

```hcl
# infra/tofu/envs/prod/variables.tf
variable "instance_type" {
  default = "t3.small"  # 2 vCPU, 2 GB RAM
}

variable "dynamodb_billing_mode" {
  default = "PROVISIONED"  # Predictable costs
}

variable "dynamodb_read_capacity" {
  default = 10
}

variable "dynamodb_write_capacity" {
  default = 10
}
```

## Configuration Checklist

Before first deployment:

- [ ] Change PROJECT_NAME in `.github/workflows/infra-deploy.yml`
- [ ] Configure AWS secrets in GitHub
- [ ] Configure Docker Hub secrets in GitHub
- [ ] Update Docker image name in workflows
- [ ] Create EC2 key pair in AWS
- [ ] Update key_name in Terraform variables
- [ ] Review instance types for each environment
- [ ] Configure CORS allowed origins
- [ ] Plan your domain name (if using Let's Encrypt)

After first deployment:

- [ ] SSH into EC2 and create .env file
- [ ] Add application secrets to .env
- [ ] Setup SSL certificate (if using domain)
- [ ] Test application endpoints
- [ ] Configure monitoring/alerts

## Troubleshooting Configuration

### Project name conflicts

Error: S3 bucket already exists

Solution: Change PROJECT_NAME to something unique

### AWS credentials invalid

Error: Unable to locate credentials

Solution: Verify AWS secrets in GitHub settings

### Docker image not found

Error: Failed to pull image

Solution: Verify Docker Hub credentials and image name

### SSH key not found

Error: Key pair 'xxx' does not exist

Solution: Create key pair in AWS EC2 console first

## Best Practices

1. Use different project names for dev and prod (e.g., `myapp-dev`, `myapp-prod`)
2. Use separate AWS accounts for dev and prod
3. Rotate AWS credentials regularly
4. Use Docker Hub access tokens, not passwords
5. Keep .env file secure, never commit to git
6. Document custom configuration changes
7. Test configuration changes in dev first
8. Use infrastructure as code for all changes
