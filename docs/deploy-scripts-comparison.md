# Deploy Scripts Comparison

## Two Different deploy.sh Scripts

There are TWO completely different `deploy.sh` scripts in this project. Understanding the difference is crucial.

## Side-by-Side Comparison

| Aspect              | Workspace deploy.sh                    | EC2 deploy.sh                  |
| ------------------- | -------------------------------------- | ------------------------------ |
| **Location**        | `scripts/deploy.sh`                    | `/opt/bi8s/scripts/deploy.sh`  |
| **Runs On**         | Your local machine                     | EC2 instance (after SSH)       |
| **Purpose**         | Deploy INFRASTRUCTURE                  | Deploy APPLICATION             |
| **What it deploys** | AWS resources (VPC, EC2, DynamoDB, S3) | Docker containers              |
| **Technology**      | Terraform/OpenTofu                     | Docker Compose                 |
| **When to use**     | Infrastructure changes                 | Application updates            |
| **Frequency**       | Rarely (infrastructure changes)        | Often (code updates)           |
| **Created by**      | You (in git repository)                | user-data.sh (during EC2 boot) |

## Workspace deploy.sh (scripts/deploy.sh)

### Purpose

Deploy or update AWS infrastructure using Terraform/OpenTofu.

### Location

```
/Users/xanderbilla/Desktop/examples-go/bi8s-go/scripts/deploy.sh
```

### Usage

```bash
# From your local machine
./scripts/deploy.sh dev plan      # Preview infrastructure changes
./scripts/deploy.sh dev apply     # Create/update AWS resources
./scripts/deploy.sh prod destroy  # Destroy infrastructure
```

### What it does

1. Validates environment (dev/prod) and action (plan/apply/destroy)
2. Changes to Terraform directory
3. Checks if backend config exists
4. Initializes Terraform if needed
5. Runs Terraform plan/apply/destroy
6. Creates or updates:
   - VPC and subnets
   - Security groups
   - EC2 instance
   - DynamoDB tables (4 tables)
   - S3 bucket
   - IAM roles

### When to execute

- First time infrastructure setup
- When you modify Terraform files (`.tf` files)
- When you need to change AWS resources
- When you want to destroy infrastructure
- Alternative to GitHub Actions for infrastructure

### Example Workflow

```bash
# 1. Initialize backend (first time only)
./scripts/init-backend.sh bi8s dev us-east-1

# 2. Plan infrastructure changes
./scripts/deploy.sh dev plan

# 3. Review the plan output

# 4. Apply changes
./scripts/deploy.sh dev apply

# 5. Get EC2 IP from output
# EC2 IP: 54.123.45.67
```

## EC2 deploy.sh (/opt/bi8s/scripts/deploy.sh)

### Purpose

Deploy or update the application (Docker containers) on EC2.

### Location

```
/opt/bi8s/scripts/deploy.sh (on EC2 instance)
```

### Usage

```bash
# SSH into EC2 first
ssh -i ~/.ssh/your-key.pem ec2-user@54.123.45.67

# Then run deploy script
cd /opt/bi8s/compose
../scripts/deploy.sh
```

### What it does

1. Checks if .env file exists (exits if not)
2. Updates IP address (calls update-ip.sh)
3. Pulls latest Docker image from Docker Hub
4. Stops old containers
5. Starts new containers with docker-compose
6. Waits for services to be healthy
7. Shows service status and logs
8. Displays access URLs

### When to execute

- First deployment after EC2 is created
- After pushing code changes (GitHub Actions builds new image)
- After updating .env file
- When you want to restart the application
- To pull and deploy latest Docker image

### Example Workflow

```bash
# 1. Make code changes locally
vim internal/http/movie_handler.go

# 2. Push to GitHub
git add .
git commit -m "Update movie handler"
git push origin dev

# 3. GitHub Actions builds and pushes Docker image
# (wait for GitHub Actions to complete)

# 4. SSH into EC2
ssh ec2-user@54.123.45.67

# 5. Deploy application
cd /opt/bi8s/compose
../scripts/deploy.sh

# 6. Application is updated!
```

## Common Confusion

### Question: "I ran ./scripts/deploy.sh but my code changes aren't deployed"

Answer: You ran the INFRASTRUCTURE deploy script. To deploy code changes:

1. Push code to GitHub (triggers Docker build)
2. SSH to EC2
3. Run `/opt/bi8s/scripts/deploy.sh` (on EC2)

### Question: "I SSH'd to EC2 and ran deploy.sh but it says command not found"

Answer: You need to provide the full path or navigate to the directory:

```bash
# Option 1: Full path
/opt/bi8s/scripts/deploy.sh

# Option 2: Navigate first
cd /opt/bi8s/compose
../scripts/deploy.sh
```

### Question: "When do I use which deploy.sh?"

Answer:

- Changed Terraform files? Use `./scripts/deploy.sh` (local)
- Changed Go code? Push to GitHub, then use `/opt/bi8s/scripts/deploy.sh` (EC2)
- Changed .env file? Use `/opt/bi8s/scripts/deploy.sh` (EC2)
- Need to create AWS resources? Use `./scripts/deploy.sh` (local)
- Need to update running application? Use `/opt/bi8s/scripts/deploy.sh` (EC2)

## Complete Deployment Flow

### Initial Setup (Infrastructure)

```bash
# On local machine
./scripts/init-backend.sh bi8s dev us-east-1
./scripts/deploy.sh dev plan
./scripts/deploy.sh dev apply
# EC2 is created with IP: 54.123.45.67
```

### First Application Deployment

```bash
# Push code to trigger Docker build
git push origin dev

# Wait for GitHub Actions to build image

# SSH to EC2
ssh ec2-user@54.123.45.67

# Configure environment
cd /opt/bi8s/compose
cp .env.example .env
vim .env  # Add secrets

# Deploy application
../scripts/deploy.sh
```

### Update Application Code

```bash
# On local machine - push code
git push origin dev

# Wait for GitHub Actions

# On EC2 - deploy
ssh ec2-user@54.123.45.67
cd /opt/bi8s/compose
../scripts/deploy.sh
```

### Update Infrastructure

```bash
# On local machine
vim infra/tofu/envs/dev/variables.tf
./scripts/deploy.sh dev plan
./scripts/deploy.sh dev apply
```

## Quick Reference

### I want to...

**Create AWS resources (VPC, EC2, DynamoDB, S3)**

```bash
./scripts/deploy.sh dev apply
```

Runs on: Local machine

**Update my Go application**

```bash
# 1. Push code
git push origin dev

# 2. Deploy on EC2
ssh ec2-user@<IP>
cd /opt/bi8s/compose && ../scripts/deploy.sh
```

Runs on: EC2 instance

**Change instance type or AWS settings**

```bash
./scripts/deploy.sh dev apply
```

Runs on: Local machine

**Restart application containers**

```bash
ssh ec2-user@<IP>
cd /opt/bi8s/compose && ../scripts/deploy.sh
```

Runs on: EC2 instance

**Destroy everything**

```bash
./scripts/deploy.sh dev destroy
```

Runs on: Local machine

## Summary

- **Workspace deploy.sh** = Infrastructure (Terraform) = Runs locally = Rare
- **EC2 deploy.sh** = Application (Docker) = Runs on EC2 = Frequent

Remember: Infrastructure changes are rare, application updates are frequent. You'll use the EC2 deploy.sh much more often than the workspace deploy.sh.
