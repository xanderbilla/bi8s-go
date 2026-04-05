# Scripts Documentation

This document describes all scripts in the project and their usage.

## Workspace Scripts (scripts/)

Located in the root `scripts/` directory. These are used for local infrastructure management.

### init-backend.sh

Initialize Terraform/OpenTofu backend (S3 bucket and DynamoDB table for state management).

Usage:

```bash
./scripts/init-backend.sh <project-name> <environment> <aws-region>
```

Example:

```bash
./scripts/init-backend.sh bi8s dev us-east-1
```

What it does:

- Validates AWS CLI installation and credentials
- Creates S3 bucket for Terraform state with:
  - Versioning enabled
  - Encryption enabled (AES256)
  - Public access blocked
  - Lifecycle policy (delete old versions after 90 days)
  - Tags for project and environment
- Creates DynamoDB table for state locking with:
  - Pay-per-request billing
  - Point-in-time recovery enabled
  - Tags for project and environment
- Generates backend configuration file at `infra/tofu/envs/<environment>/backend-<environment>.hcl`

When to use:

- First time setting up infrastructure for an environment
- Before running `tofu init` or `terraform init`
- Only needs to be run once per environment

### deploy.sh

Deploy infrastructure using OpenTofu/Terraform.

Usage:

```bash
./scripts/deploy.sh <environment> [plan|apply|destroy]
```

Examples:

```bash
./scripts/deploy.sh dev plan      # Plan changes for dev
./scripts/deploy.sh dev apply     # Apply changes to dev
./scripts/deploy.sh prod destroy  # Destroy prod infrastructure
```

What it does:

- Validates environment (dev or prod)
- Validates action (plan, apply, or destroy)
- Changes to environment directory
- Checks if backend config exists
- Detects if tofu or terraform is installed
- Initializes backend if needed
- Executes the requested action:
  - plan: Creates execution plan and saves to file
  - apply: Applies changes (uses saved plan if exists)
  - destroy: Destroys all resources (asks for confirmation)

When to use:

- After running init-backend.sh
- To preview infrastructure changes (plan)
- To create/update infrastructure (apply)
- To tear down infrastructure (destroy)

## Infrastructure Scripts (infra/scripts/)

Located in `infra/scripts/`. These are helper scripts for EC2 deployment and management from your local machine.

### build-and-deploy.sh

Build Go binary locally and deploy to EC2 (non-Docker deployment).

Usage:

```bash
./infra/scripts/build-and-deploy.sh <environment> <ec2-ip>
```

Example:

```bash
./infra/scripts/build-and-deploy.sh dev 54.123.45.67
```

What it does:

- Builds Go binary for Linux (CGO_ENABLED=0 GOOS=linux GOARCH=amd64)
- Copies binary to EC2 via SCP to `/opt/bi8s/`
- Restarts systemd service on EC2
- Checks service status
- Cleans up local binary

When to use:

- For non-Docker deployments
- Quick binary updates without rebuilding Docker image
- Testing changes directly on EC2

Note: This is an alternative to Docker-based deployment. The current setup uses Docker Compose.

### docker-deploy.sh

Build Docker image locally and deploy to EC2 (alternative to Docker Hub workflow).

Usage:

```bash
./infra/scripts/docker-deploy.sh <environment> <ec2-ip>
```

Example:

```bash
./infra/scripts/docker-deploy.sh dev 54.123.45.67
```

What it does:

- Builds Docker image locally using `infra/docker/Dockerfile.deploy`
- Tags image with timestamp and 'latest'
- Saves image as tar.gz file
- Copies tar.gz to EC2 via SCP
- Copies docker-compose.yml to EC2
- Loads image on EC2
- Stops existing containers
- Starts new containers with docker-compose
- Shows container status and logs
- Cleans up local tar.gz file

When to use:

- When you don't want to push to Docker Hub
- For private/local deployments
- Testing Docker images before publishing
- When Docker Hub is unavailable

Note: The recommended workflow uses GitHub Actions to build and push to Docker Hub.

### setup-ssl-letsencrypt.sh

Setup Let's Encrypt SSL certificate on EC2 for a domain.

Usage:

```bash
./infra/scripts/setup-ssl-letsencrypt.sh <ec2-ip> <domain>
```

Example:

```bash
./infra/scripts/setup-ssl-letsencrypt.sh 54.123.45.67 api.yourdomain.com
```

What it does:

- Verifies DNS A record points to EC2 IP (using dig)
- Warns if DNS doesn't match (allows override)
- SSHs into EC2 and runs `/opt/bi8s/scripts/renew-ssl.sh`
- Restarts nginx container
- Shows service status

When to use:

- After pointing your domain to EC2 IP
- For production deployments with custom domain
- To replace self-signed certificate with Let's Encrypt

Prerequisites:

- Domain DNS A record pointing to EC2 IP
- Ports 80 and 443 open in security group
- EC2 instance running with nginx

### update-ec2-configs.sh

Update configuration files on EC2 without recreating the instance.

Usage:

```bash
./infra/scripts/update-ec2-configs.sh <ec2-ip>
```

Example:

```bash
./infra/scripts/update-ec2-configs.sh 54.123.45.67
```

What it does:

- SSHs into EC2
- Clones latest repository to /tmp
- Backs up current configs using backup-config.sh
- Updates helper scripts from infra/scripts/
- Updates docker-compose.yml
- Updates nginx configuration
- Sets proper permissions
- Restarts nginx container
- Shows service status
- Cleans up temporary files

When to use:

- After updating scripts in the repository
- After changing docker-compose configuration
- After updating nginx configuration
- To sync EC2 configs with repository without full redeployment

## EC2 Scripts (on EC2 instance)

These scripts are created on the EC2 instance during initialization (via user-data.sh) and located at `/opt/bi8s/scripts/`.

### deploy.sh

Deploy or update application on EC2 (runs on EC2, not locally).

Usage (on EC2):

```bash
cd /opt/bi8s/compose
../scripts/deploy.sh
```

What it does:

- Checks if .env file exists (exits with error if not)
- Runs update-ip.sh to update IP address
- Pulls latest Docker image from Docker Hub
- Stops old containers
- Starts new containers with docker-compose
- Waits for services to be healthy
- Shows service status
- Shows recent logs
- Displays access URLs and health check command

When to use:

- First time deployment after EC2 creation
- After new Docker image is pushed to Docker Hub
- After updating .env file
- To restart application with latest image

### update-ip.sh

Update public IP address and regenerate SSL certificate (runs on EC2).

Usage (on EC2):

```bash
/opt/bi8s/scripts/update-ip.sh
```

What it does:

- Fetches current public IP from EC2 metadata service
- Reads old IP from .env file
- If IP changed:
  - Updates PUBLIC_IP in .env file
  - Updates PUBLIC_IP in /etc/profile.d/bi8s.sh
  - Regenerates self-signed SSL certificate with new IP
  - Sets proper permissions on certificate files
- If IP unchanged, does nothing

When to use:

- Automatically runs on EC2 restart (via systemd service)
- Manually after EC2 stop/start
- After Elastic IP association/disassociation

Note: This script runs automatically before docker-compose starts via systemd service.

### renew-ssl.sh

Setup Let's Encrypt SSL certificate for a domain (runs on EC2).

Usage (on EC2):

```bash
/opt/bi8s/scripts/renew-ssl.sh <domain>
```

Example (on EC2):

```bash
/opt/bi8s/scripts/renew-ssl.sh api.yourdomain.com
```

What it does:

- Installs certbot if not present
- Stops nginx container
- Obtains certificate using certbot webroot method
- Copies certificates to /opt/bi8s/nginx/ssl/live/
- Backs up certificates to /opt/bi8s/nginx/ssl/archive/
- Sets proper permissions
- Restarts nginx container

When to use:

- After pointing domain to EC2
- To replace self-signed certificate
- For production deployments
- Called by setup-ssl-letsencrypt.sh from local machine

Prerequisites:

- Domain DNS pointing to EC2 IP
- Ports 80 and 443 accessible
- Nginx container running

### backup-config.sh

Backup configuration files (runs on EC2).

Usage (on EC2):

```bash
/opt/bi8s/scripts/backup-config.sh
```

What it does:

- Creates timestamped backup directory at `/opt/bi8s/backups/<timestamp>/`
- Backs up compose directory (docker-compose.yml, .env)
- Backs up nginx configuration (conf.d/)
- Backs up SSL certificates (ssl/)
- Shows backup location and contents

When to use:

- Before making configuration changes
- Before running update-ec2-configs.sh
- Regular backups (can be scheduled with cron)
- Before major updates

## Script Execution Flow

### Initial Setup

```
1. Local: ./scripts/init-backend.sh bi8s dev us-east-1
   Creates S3 bucket and DynamoDB table for state

2. Local: ./scripts/deploy.sh dev plan
   Plans infrastructure changes

3. Local: ./scripts/deploy.sh dev apply
   Creates AWS resources including EC2

4. EC2: user-data.sh runs automatically
   Installs Docker, creates directory structure, generates scripts

5. EC2: SSH into instance
   ssh ec2-user@<EC2_IP>

6. EC2: cd /opt/bi8s/compose && cp .env.example .env && vim .env
   Configure environment variables

7. EC2: ../scripts/deploy.sh
   Deploy application
```

### Update Application Code

```
1. Local: git push origin dev
   Triggers GitHub Actions

2. GitHub Actions: Builds and pushes Docker image

3. EC2: cd /opt/bi8s/compose && ../scripts/deploy.sh
   Pulls new image and restarts containers
```

### Update Infrastructure

```
1. Local: Edit infra/tofu/envs/dev/*.tf files

2. Local: git push origin dev
   Triggers GitHub Actions to update infrastructure

   OR

   Local: ./scripts/deploy.sh dev plan
   Local: ./scripts/deploy.sh dev apply
```

### Update Configurations

```
1. Local: Edit infra/scripts/*.sh or infra/docker/*.yml

2. Local: ./infra/scripts/update-ec2-configs.sh <EC2_IP>
   Updates configs on EC2 without redeployment
```

### Setup SSL Certificate

```
1. Local: Point domain DNS to EC2 IP

2. Local: ./infra/scripts/setup-ssl-letsencrypt.sh <EC2_IP> api.yourdomain.com
   Installs Let's Encrypt certificate
```

## Common Issues

### Backend not initialized

Error: Backend config not found

Solution:

```bash
./scripts/init-backend.sh bi8s dev us-east-1
```

### .env file not found on EC2

Error: .env file not found when running deploy.sh

Solution:

```bash
cd /opt/bi8s/compose
cp .env.example .env
vim .env  # Add your secrets
```

### IP changed after restart

Issue: Application not accessible after EC2 restart

Solution: IP is automatically updated via systemd service. If manual update needed:

```bash
/opt/bi8s/scripts/update-ip.sh
docker-compose restart
```

### SSL certificate expired

Issue: Let's Encrypt certificate expired

Solution:

```bash
/opt/bi8s/scripts/renew-ssl.sh api.yourdomain.com
```

### Configs out of sync

Issue: EC2 configs don't match repository

Solution:

```bash
./infra/scripts/update-ec2-configs.sh <EC2_IP>
```

## Security Notes

- Scripts use SSH key authentication (no passwords)
- AWS credentials required for init-backend.sh and deploy.sh
- EC2 uses IAM roles (no AWS keys on instance)
- Secrets stored in .env file (not in git)
- SSL certificates have proper permissions (644 for cert, 600 for key)
- Backup configs before making changes

## Best Practices

- Always run `plan` before `apply`
- Backup configs before updates
- Test in dev before prod
- Use GitHub Actions for CI/CD
- Keep scripts executable: `chmod +x script.sh`
- Review script output for errors
- Use version control for infrastructure changes
