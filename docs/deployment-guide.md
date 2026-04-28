# Deployment Guide

Complete guide for deploying the bi8s application to AWS using OpenTofu and GitHub Actions.

## Health & readiness

The service exposes three probes under `/v1`:

- `GET /v1/livez` — process liveness; no dependencies, no environment metadata.
- `GET /v1/readyz` — returns `503 NOT_READY` until `main.go` flips the readiness
  flag once the listener is bound, and `503 SERVICE_UNAVAILABLE` (with a
  `details.checks` map) if any registered dependency probe fails. Wire this to
  your load balancer / Kubernetes readiness probe.
- `GET /v1/health` — full dependency check; safe for dashboards.

On `SIGINT`/`SIGTERM` the service first marks itself **not ready** so the load
balancer drains traffic before `Server.Shutdown` is called.

## API documentation

After deploying, the OpenAPI spec is served at `GET /v1/openapi.yaml` and a
Swagger UI viewer at `GET /v1/docs`. Both are public — restrict at the network
layer if you need to hide them.

## Scripts Overview

### Workspace Scripts (scripts/)

Located in the root `scripts/` directory, these are used for local infrastructure management:

- `init-backend.sh` - Initialize Terraform/OpenTofu backend (S3 bucket and DynamoDB table for state management)
  - Usage: `./scripts/init-backend.sh <project-name> <environment> <aws-region>`
  - Example: `./scripts/init-backend.sh bi8s dev us-east-1`
  - Creates S3 bucket with versioning and encryption
  - Creates DynamoDB table for state locking
  - Generates backend configuration file

- `deploy.sh` - Deploy INFRASTRUCTURE using OpenTofu/Terraform (NOT application)
  - Usage: `./scripts/deploy.sh <environment> [plan|apply|destroy]`
  - Example: `./scripts/deploy.sh dev plan`
  - Runs on: Local machine
  - Purpose: Create/update AWS resources (VPC, EC2, DynamoDB, S3, IAM)
  - Validates environment and action
  - Initializes backend if needed
  - Executes plan, apply, or destroy
  - NOTE: This is different from the EC2 deploy.sh script

### Infrastructure Scripts (infra/scripts/)

Located in `infra/scripts/`, these are helper scripts for EC2 deployment and management:

- `build-and-deploy.sh` - Build Go binary and deploy to EC2
  - Usage: `./infra/scripts/build-and-deploy.sh <environment> <ec2-ip>`
  - Builds Go binary for Linux
  - Copies binary to EC2 via SCP
  - Restarts systemd service
  - Note: This is for non-Docker deployments

- `docker-deploy.sh` - Build Docker image and deploy to EC2
  - Usage: `./infra/scripts/docker-deploy.sh <environment> <ec2-ip>`
  - Builds Docker image locally
  - Saves image as tar.gz
  - Copies to EC2 and loads image
  - Restarts Docker Compose services
  - Note: Alternative to Docker Hub workflow

- `setup-ssl-letsencrypt.sh` - Setup Let's Encrypt SSL certificate
  - Usage: `./infra/scripts/setup-ssl-letsencrypt.sh <ec2-ip> <domain>`
  - Verifies DNS points to EC2
  - Runs SSL setup script on EC2
  - Restarts Nginx with new certificate

- `update-ec2-configs.sh` - Update configuration files on EC2
  - Usage: `./infra/scripts/update-ec2-configs.sh <ec2-ip>`
  - Clones latest repository
  - Backs up current configs
  - Updates scripts, docker-compose, and nginx configs
  - Restarts services

### EC2 Scripts (on EC2 instance)

These scripts are created on the EC2 instance during initialization (via user-data.sh) and located at `/opt/bi8s/scripts/`:

- `deploy.sh` - Deploy or update APPLICATION on EC2 (NOT infrastructure)
  - Usage (on EC2): `cd /opt/bi8s/compose && ../scripts/deploy.sh`
  - Runs on: EC2 instance (after SSH)
  - Purpose: Deploy/update Docker containers
  - Checks if .env file exists
  - Updates IP address
  - Pulls latest Docker image
  - Restarts containers
  - Shows status and logs
  - NOTE: This is different from the workspace deploy.sh script

- `update-ip.sh` - Update public IP address
  - Fetches current public IP from metadata service
  - Updates .env file
  - Regenerates self-signed SSL certificate
  - Runs automatically on EC2 restart via systemd

- `renew-ssl.sh` - Setup Let's Encrypt SSL certificate
  - Installs certbot if needed
  - Obtains certificate for domain
  - Copies certificates to nginx directory
  - Restarts nginx container

- `backup-config.sh` - Backup configuration files
  - Backs up docker-compose files
  - Backs up nginx configs
  - Backs up SSL certificates
  - Creates timestamped backup directory

## Prerequisites

- AWS account with appropriate permissions
- GitHub repository with Actions enabled
- Docker Hub account
- SSH key pair for EC2 access

## Setup GitHub Secrets

Add these secrets in your GitHub repository settings (Settings → Secrets and variables → Actions):

```
AWS_ACCESS_KEY_ID=<your-aws-access-key>
AWS_SECRET_ACCESS_KEY=<your-aws-secret-key>
AWS_REGION=us-east-1
DOCKER_REGISTRY=docker.io
DOCKER_USERNAME=<your-dockerhub-username>
DOCKER_PASSWORD=<your-dockerhub-token>
```

## Deployment Workflow

### 1. Infrastructure Deployment

Infrastructure is automatically deployed when you push changes to the `infra/` folder:

```bash
# Make infrastructure changes
vim infra/tofu/envs/dev/variables.tf

# Commit and push
git add infra/
git commit -m "Update infrastructure"
git push origin dev
```

GitHub Actions will:

- Create/update VPC, subnets, security groups
- Create/update DynamoDB tables (4 tables)
- Create/update S3 bucket
- Create/update IAM roles
- Create/update EC2 instance with Docker pre-installed
- Output the EC2 public IP

### 2. Application Deployment

Application Docker images are automatically built when you push code changes:

```bash
# Make code changes
vim internal/http/movie_handler.go

# Commit and push
git add .
git commit -m "Update movie handler"
git push origin dev
```

GitHub Actions will:

- Build multi-platform Docker image (amd64, arm64)
- Push to Docker Hub with tags: `latest` and `<commit-sha>`

### 3. Deploy to EC2

After infrastructure and image are ready, SSH into EC2 and deploy:

```bash
# Get EC2 IP from GitHub Actions output or AWS Console
ssh -i ~/.ssh/your-key.pem ec2-user@<EC2_IP>

# Navigate to application directory
cd /opt/bi8s/compose

# First time: Create .env file
cp .env.example .env
vim .env  # Add your custom secrets (see below)

# Deploy application
../scripts/deploy.sh
```

The deploy script will:

- Update IP address
- Pull latest Docker image
- Stop old containers
- Start new containers
- Show service status and logs

## Environment Configuration

### Auto-configured Variables

These are automatically set by Terraform in `.env`:

```bash
PROJECT_NAME=bi8s
IMAGE_NAME=xanderbilla/my-project:latest
APP_ENV=dev
AWS_REGION=us-east-1
DYNAMODB_MOVIE_TABLE=bi8s-content-table-dev
DYNAMODB_PERSON_TABLE=bi8s-person-table-dev
DYNAMODB_ATTRIBUTE_TABLE=bi8s-attributes-table-dev
DYNAMODB_ENCODER_TABLE=bi8s-video-table-dev
S3_BUCKET=bi8s-storage-dev
CORS_ALLOWED_ORIGINS=*
CORS_ALLOW_PRIVATE_NETWORK=true
PUBLIC_IP=<auto-detected>
```

### Custom Variables

Add your application-specific secrets to `.env`:

```bash
# Example: JWT Secret
JWT_SECRET=your-secret-key-here

# Example: API Keys
STRIPE_API_KEY=sk_test_xxxxx
SENDGRID_API_KEY=SG.xxxxx

# Example: Database (if using external DB)
DB_HOST=localhost
DB_PORT=5432
DB_USER=myuser
DB_PASSWORD=mypassword

# Example: Redis
REDIS_URL=redis://localhost:6379
```

## Directory Structure on EC2

```
/opt/bi8s/
├── compose/
│   ├── docker-compose.yml       # Container orchestration
│   ├── .env                     # Your environment variables
│   └── .env.example             # Template with all variables
├── nginx/
│   ├── conf.d/
│   │   └── api.conf             # Nginx configuration
│   ├── ssl/
│   │   ├── live/                # Active certificates
│   │   │   ├── cert.crt
│   │   │   └── cert.key
│   │   ├── archive/             # Certificate backups
│   │   └── renewal/             # Renewal configs
│   └── certbot/
│       └── www/                 # ACME challenge files
└── scripts/
    ├── deploy.sh                # Deploy/update application
    ├── update-ip.sh             # Update IP (auto-runs on restart)
    ├── renew-ssl.sh             # Setup Let's Encrypt SSL
    └── backup-config.sh         # Backup configuration
```

## SSL Certificate Setup

### Option 1: Self-Signed (Default)

Self-signed certificates are automatically generated on EC2 boot. Access via:

```bash
https://<EC2_IP>/v1/health
```

Note: Browsers will show a security warning. This is fine for development.

### Option 2: Let's Encrypt (Production)

For production with a domain name:

```bash
# 1. Point your domain to EC2 IP
# Add DNS A record: api.yourdomain.com → <EC2_IP>

# 2. SSH into EC2
ssh ec2-user@<EC2_IP>

# 3. Run SSL setup script
/opt/bi8s/scripts/renew-ssl.sh api.yourdomain.com

# 4. Access via domain
curl https://api.yourdomain.com/v1/health
```

The script will:

- Install certbot
- Obtain Let's Encrypt certificate
- Update Nginx configuration
- Restart Nginx container

## Common Operations

### View Logs

```bash
cd /opt/bi8s/compose
docker-compose logs -f          # All services
docker-compose logs -f api      # API only
docker-compose logs -f nginx    # Nginx only
```

### Check Service Status

```bash
cd /opt/bi8s/compose
docker-compose ps
```

### Restart Services

```bash
cd /opt/bi8s/compose
docker-compose restart
```

### Update Application

```bash
# After pushing code changes and GitHub Actions builds new image
cd /opt/bi8s/compose
../scripts/deploy.sh
```

### Update Configuration

```bash
cd /opt/bi8s/compose
vim .env
../scripts/deploy.sh
```

### Backup Configuration

```bash
/opt/bi8s/scripts/backup-config.sh
```

### Manual IP Update

```bash
# Automatically runs on EC2 restart, but can be run manually
/opt/bi8s/scripts/update-ip.sh
```

## Troubleshooting

### Service Won't Start

```bash
# Check logs
docker-compose logs -f

# Check if .env exists
ls -la /opt/bi8s/compose/.env

# Verify environment variables
cat /opt/bi8s/compose/.env
```

### Can't Access API

```bash
# Check if containers are running
docker-compose ps

# Check Nginx logs
docker-compose logs nginx

# Check security group allows ports 80, 443
# AWS Console → EC2 → Security Groups
```

### SSL Certificate Issues

```bash
# Check certificate files
ls -la /opt/bi8s/nginx/ssl/live/

# Regenerate self-signed certificate
/opt/bi8s/scripts/update-ip.sh

# For Let's Encrypt issues
docker-compose logs nginx
```

### IP Changed After Restart

```bash
# IP is automatically updated on restart via systemd service
# To manually update:
/opt/bi8s/scripts/update-ip.sh
docker-compose restart
```

### DynamoDB Access Issues

```bash
# Verify IAM role is attached to EC2
aws sts get-caller-identity

# Check if tables exist
aws dynamodb list-tables --region us-east-1
```

## Environment-Specific Deployments

### Development (dev branch)

```bash
git push origin dev
```

- Deploys to dev environment
- Uses dev DynamoDB tables
- Uses dev S3 bucket
- Lower capacity/cheaper resources

### Production (main branch)

```bash
git push origin main
```

- Deploys to prod environment
- Uses prod DynamoDB tables
- Uses prod S3 bucket
- Higher capacity/production-grade resources

## Manual Infrastructure Operations

### Plan Changes

```bash
cd infra/tofu/envs/dev
tofu init -backend-config=backend-dev.hcl
tofu plan
```

### Apply Changes

```bash
tofu apply
```

### Destroy Infrastructure

```bash
# Via GitHub Actions (recommended)
# Go to Actions → Infrastructure Deployment → Run workflow
# Select environment and action: destroy

# Or manually
cd infra/tofu/envs/dev
tofu destroy
```

## Monitoring

### Health Check

```bash
curl https://<EC2_IP>/v1/health
```

### Resource Usage

```bash
# Container stats
docker stats

# Disk usage
df -h

# Memory usage
free -h

# Network connections
netstat -tlnp
```

## Security Best Practices

- Use IAM roles (no AWS keys on EC2)
- Enable HTTPS (SSL/TLS)
- Restrict security group rules
- Keep secrets in `.env` (not in git)
- Use Let's Encrypt for production
- Regular security updates
- Monitor access logs

## Cost Optimization

- Use `t3.micro` or `t3.small` for dev
- Use DynamoDB on-demand billing for variable workloads
- Enable S3 lifecycle policies (already configured)
- Stop EC2 instances when not in use (dev only)
- Use reserved instances for production

## Support

For issues or questions:

- Check logs: `docker-compose logs -f`
- Review documentation: `docs/`
- Check GitHub Actions runs
- Verify AWS resources in console
