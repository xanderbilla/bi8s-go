# Deployment Workflow

This document explains the complete deployment workflow and when each script is used.

## Important: Two Different deploy.sh Scripts

There are TWO different `deploy.sh` scripts with completely different purposes:

### 1. Workspace deploy.sh (scripts/deploy.sh)

**Location:** `scripts/deploy.sh` (in your local workspace)

**Purpose:** Deploy INFRASTRUCTURE (AWS resources) using Terraform/OpenTofu

**Runs on:** Your local machine

**What it does:**

- Deploys VPC, EC2, DynamoDB, S3, IAM roles
- Runs Terraform plan/apply/destroy
- Creates or updates AWS infrastructure

**Usage:**

```bash
./scripts/deploy.sh dev plan      # Plan infrastructure changes
./scripts/deploy.sh dev apply     # Apply infrastructure changes
./scripts/deploy.sh prod destroy  # Destroy infrastructure
```

**When to use:**

- First time infrastructure setup
- When you change Terraform files
- When you need to update AWS resources
- Alternative to GitHub Actions for infrastructure

### 2. EC2 deploy.sh (/opt/bi8s/scripts/deploy.sh)

**Location:** `/opt/bi8s/scripts/deploy.sh` (on EC2 instance)

**Purpose:** Deploy APPLICATION (Docker containers)

**Runs on:** EC2 instance (after SSH)

**What it does:**

- Pulls latest Docker image from Docker Hub
- Stops old containers
- Starts new containers
- Shows application status

**Usage:**

```bash
# SSH into EC2 first
ssh ec2-user@<EC2_IP>

# Then run deploy script
cd /opt/bi8s/compose
../scripts/deploy.sh
```

**When to use:**

- After pushing code changes and GitHub Actions builds new image
- When you want to update the running application
- After changing .env file
- To restart the application

## Quick Reference

| Script                        | Location       | Purpose        | Runs On       |
| ----------------------------- | -------------- | -------------- | ------------- |
| `scripts/deploy.sh`           | Workspace root | Infrastructure | Local machine |
| `/opt/bi8s/scripts/deploy.sh` | EC2 instance   | Application    | EC2 instance  |

## Overview

The project uses a hybrid deployment approach:

- Infrastructure managed by OpenTofu/Terraform
- Application deployed via Docker containers
- CI/CD automated through GitHub Actions
- Manual deployment control on EC2

## Workflow Scenarios

### Scenario 1: Initial Setup (First Time Deployment)

This is when you're setting up everything from scratch.

#### Step 0: Configure Project Name (Before Anything Else)

Update the project name in `.github/workflows/infra-deploy.yml`:

```yaml
env:
  PROJECT_NAME: "your-project-name" # Change from "bi8s" to your name
```

This ensures all AWS resources are named correctly.

#### Step 1: Initialize Backend (Local Machine)

```bash
./scripts/init-backend.sh bi8s dev us-east-1
```

**Script Used:** `scripts/init-backend.sh`

**What it does:**

- Creates S3 bucket for Terraform state storage
- Creates DynamoDB table for state locking
- Generates backend configuration file

**When to use:** Only once per environment (dev/prod) before any infrastructure deployment

#### Step 2: Deploy Infrastructure (Local Machine or GitHub Actions)

Option A - Using GitHub Actions (Recommended):

```bash
git add infra/
git commit -m "Initial infrastructure"
git push origin dev
```

Option B - Using local script:

```bash
./scripts/deploy.sh dev plan
./scripts/deploy.sh dev apply
```

**Script Used:** `scripts/deploy.sh` or GitHub Actions workflow

**What it does:**

- Creates VPC, subnets, security groups
- Creates DynamoDB tables (4 tables)
- Creates S3 bucket for file storage
- Creates IAM roles for EC2
- Launches EC2 instance
- Runs user-data.sh on EC2 (installs Docker, creates directory structure, generates helper scripts)

**When to use:** First time setup or when infrastructure changes are needed

#### Step 3: Configure Application (On EC2)

```bash
# SSH into EC2
ssh -i ~/.ssh/your-key.pem ec2-user@<EC2_IP>

# Navigate to compose directory
cd /opt/bi8s/compose

# Create .env from example
cp .env.example .env

# Edit .env with your secrets
vim .env
```

**Script Used:** None (manual configuration)

**What it does:**

- Creates environment file with application secrets
- Adds JWT secrets, API keys, database credentials, etc.

**When to use:** After EC2 is created, before first deployment

#### Step 4: Deploy Application (On EC2)

```bash
cd /opt/bi8s/compose
../scripts/deploy.sh
```

**Script Used:** `/opt/bi8s/scripts/deploy.sh` (runs on EC2)

**What it does:**

- Checks if .env file exists
- Updates IP address (calls update-ip.sh)
- Pulls latest Docker image from Docker Hub
- Stops old containers
- Starts new containers
- Shows status and logs

**When to use:** First deployment and every time you want to update the application

---

### Scenario 2: Update Application Code

This is the most common workflow - you've made code changes and want to deploy them.

#### Step 1: Push Code Changes (Local Machine)

```bash
git add .
git commit -m "Update movie handler"
git push origin dev
```

**Script Used:** None (triggers GitHub Actions)

**What it does:**

- GitHub Actions detects changes in `cmd/**` or `internal/**`
- Builds multi-platform Docker image
- Pushes image to Docker Hub with tags: `latest` and `<commit-sha>`

**When to use:** Every time you make code changes

#### Step 2: Deploy to EC2 (On EC2)

```bash
ssh ec2-user@<EC2_IP>
cd /opt/bi8s/compose
../scripts/deploy.sh
```

**Script Used:** `/opt/bi8s/scripts/deploy.sh` (runs on EC2)

**What it does:**

- Pulls the new Docker image from Docker Hub
- Restarts containers with new image
- Shows deployment status

**When to use:** After GitHub Actions finishes building the image

**Alternative:** If you don't want to use Docker Hub, you can use:

```bash
# From local machine
./infra/scripts/docker-deploy.sh dev <EC2_IP>
```

**Script Used:** `infra/scripts/docker-deploy.sh`

**What it does:**

- Builds Docker image locally
- Saves image as tar.gz
- Copies to EC2 via SCP
- Loads image on EC2
- Restarts containers

**When to use:** When you want to deploy without pushing to Docker Hub (testing, private deployments)

---

### Scenario 3: Update Infrastructure

You've changed Terraform files and need to update AWS resources.

#### Option A: Using GitHub Actions (Recommended)

```bash
vim infra/tofu/envs/dev/variables.tf
git add infra/
git commit -m "Update instance type"
git push origin dev
```

**Script Used:** GitHub Actions workflow

**What it does:**

- Detects changes in `infra/**`
- Runs terraform plan
- Applies changes automatically
- Outputs new EC2 IP if changed

**When to use:** For tracked infrastructure changes

#### Option B: Using Local Script

```bash
./scripts/deploy.sh dev plan
./scripts/deploy.sh dev apply
```

**Script Used:** `scripts/deploy.sh`

**What it does:**

- Plans infrastructure changes
- Shows what will be created/modified/destroyed
- Applies changes when you run with `apply`

**When to use:** For quick infrastructure updates or testing

---

### Scenario 4: Update Configuration Files

You've updated scripts, docker-compose.yml, or nginx.conf and need to sync them to EC2.

```bash
# From local machine
./infra/scripts/update-ec2-configs.sh <EC2_IP>
```

**Script Used:** `infra/scripts/update-ec2-configs.sh`

**What it does:**

- Clones latest repository to EC2
- Backs up current configs (calls backup-config.sh)
- Updates helper scripts from `infra/scripts/`
- Updates docker-compose.yml
- Updates nginx configuration
- Restarts nginx container
- Shows service status

**When to use:**

- After updating scripts in the repository
- After changing docker-compose configuration
- After updating nginx configuration
- To sync EC2 configs with repository without full redeployment

---

### Scenario 5: Setup SSL Certificate (Production)

You have a domain and want to use Let's Encrypt instead of self-signed certificate.

#### Step 1: Point Domain to EC2

Update your DNS A record:

```
api.yourdomain.com → <EC2_IP>
```

#### Step 2: Setup SSL (From Local Machine)

```bash
./infra/scripts/setup-ssl-letsencrypt.sh <EC2_IP> api.yourdomain.com
```

**Script Used:** `infra/scripts/setup-ssl-letsencrypt.sh`

**What it does:**

- Verifies DNS points to EC2 IP
- SSHs into EC2
- Calls `/opt/bi8s/scripts/renew-ssl.sh` on EC2
- Restarts nginx with new certificate

**When to use:** After pointing domain to EC2, for production deployments

**What happens on EC2:**

**Script Used:** `/opt/bi8s/scripts/renew-ssl.sh` (runs on EC2)

**What it does:**

- Installs certbot if not present
- Stops nginx container
- Obtains Let's Encrypt certificate
- Copies certificates to nginx directory
- Restarts nginx container

**When to use:** Called by setup-ssl-letsencrypt.sh or manually for certificate renewal

---

### Scenario 6: EC2 Restart (Automatic)

EC2 instance is stopped and started (IP address changes).

**Script Used:** `/opt/bi8s/scripts/update-ip.sh` (runs automatically via systemd)

**What it does:**

- Fetches new public IP from EC2 metadata service
- Compares with old IP in .env file
- If changed:
  - Updates .env file with new IP
  - Updates environment profile
  - Regenerates self-signed SSL certificate with new IP
  - Sets proper permissions

**When to use:** Runs automatically on every EC2 restart via systemd service

**Manual usage:**

```bash
# On EC2
/opt/bi8s/scripts/update-ip.sh
docker-compose restart
```

---

### Scenario 7: Backup Configuration

Before making major changes, you want to backup current configs.

```bash
# On EC2
/opt/bi8s/scripts/backup-config.sh
```

**Script Used:** `/opt/bi8s/scripts/backup-config.sh` (runs on EC2)

**What it does:**

- Creates timestamped backup directory
- Backs up docker-compose files and .env
- Backs up nginx configuration
- Backs up SSL certificates
- Shows backup location

**When to use:**

- Before making configuration changes
- Before running update-ec2-configs.sh
- Regular backups (can be scheduled with cron)
- Before major updates

---

### Scenario 8: Non-Docker Deployment (Alternative)

You want to deploy Go binary directly without Docker.

```bash
# From local machine
./infra/scripts/build-and-deploy.sh dev <EC2_IP>
```

**Script Used:** `infra/scripts/build-and-deploy.sh`

**What it does:**

- Builds Go binary for Linux
- Copies binary to EC2 via SCP
- Restarts systemd service on EC2
- Shows service status

**When to use:**

- For non-Docker deployments
- Quick binary updates without rebuilding Docker image
- Testing changes directly on EC2

**Note:** This is an alternative approach. The current setup uses Docker Compose.

---

## Script Usage Summary

### Local Machine Scripts

| Script                                   | When to Use                              | Frequency    |
| ---------------------------------------- | ---------------------------------------- | ------------ |
| `scripts/init-backend.sh`                | First time setup, create S3 and DynamoDB | Once         |
| `scripts/deploy.sh`                      | Deploy/update infrastructure             | As needed    |
| `infra/scripts/build-and-deploy.sh`      | Non-Docker deployment (alternative)      | Rarely       |
| `infra/scripts/docker-deploy.sh`         | Deploy without Docker Hub (alternative)  | Occasionally |
| `infra/scripts/setup-ssl-letsencrypt.sh` | Setup Let's Encrypt SSL                  | Once/renewal |
| `infra/scripts/update-ec2-configs.sh`    | Sync configs to EC2                      | As needed    |

### EC2 Scripts (Run on EC2)

| Script                               | When to Use                               | Frequency    |
| ------------------------------------ | ----------------------------------------- | ------------ |
| `/opt/bi8s/scripts/deploy.sh`        | Deploy/update application                 | Often        |
| `/opt/bi8s/scripts/update-ip.sh`     | Update IP (runs automatically on restart) | Automatic    |
| `/opt/bi8s/scripts/renew-ssl.sh`     | Setup/renew Let's Encrypt certificate     | Once/renewal |
| `/opt/bi8s/scripts/backup-config.sh` | Backup configurations                     | As needed    |

### GitHub Actions (Automatic)

| Workflow             | Triggers                             | What it Does                |
| -------------------- | ------------------------------------ | --------------------------- |
| `infra-deploy.yml`   | Changes in `infra/**`                | Deploy infrastructure       |
| `docker-publish.yml` | Changes in `cmd/**` or `internal/**` | Build and push Docker image |

## Recommended Workflow

### Development

1. Make code changes locally
2. Push to `dev` branch
3. GitHub Actions builds Docker image
4. SSH to EC2 and run `/opt/bi8s/scripts/deploy.sh`
5. Test changes

### Production

1. Merge to `main` branch
2. GitHub Actions deploys to prod infrastructure
3. GitHub Actions builds Docker image
4. SSH to prod EC2 and run `/opt/bi8s/scripts/deploy.sh`
5. Verify deployment

### Configuration Updates

1. Update configs in repository
2. Run `./infra/scripts/update-ec2-configs.sh <EC2_IP>`
3. Verify changes

### SSL Setup (One Time)

1. Point domain to EC2
2. Run `./infra/scripts/setup-ssl-letsencrypt.sh <EC2_IP> domain.com`
3. Certificate auto-renews

## Troubleshooting

### Application won't start

Check logs:

```bash
cd /opt/bi8s/compose
docker-compose logs -f
```

### IP changed after restart

IP is automatically updated. If manual update needed:

```bash
/opt/bi8s/scripts/update-ip.sh
docker-compose restart
```

### Configs out of sync

Sync from repository:

```bash
./infra/scripts/update-ec2-configs.sh <EC2_IP>
```

### Need to rollback

Restore from backup:

```bash
# On EC2
ls /opt/bi8s/backups/
# Copy files from backup directory
```

## Best Practices

1. Always backup before major changes
2. Test in dev before prod
3. Use GitHub Actions for CI/CD
4. Keep .env file secure (never commit)
5. Monitor logs after deployment
6. Use Let's Encrypt for production
7. Regular backups (schedule with cron)
8. Document custom changes
