# Deployment

This project uses Infrastructure as Code (OpenTofu/Terraform) for AWS deployment with automated CI/CD via GitHub Actions.

## Quick Links

- [Complete Deployment Guide](deployment-guide.md) - Full deployment instructions
- [GitHub Workflows](../.github/workflows/README.md) - CI/CD documentation

## Architecture

```
┌─────────────────────────────────────────┐
│ GitHub Actions                          │
│ ├─ infra-deploy.yml (Infrastructure)    │
│ └─ docker-publish.yml (Application)     │
└─────────────────────────────────────────┘
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
┌──────────────┐        ┌──────────────┐
│ AWS          │        │ Docker Hub   │
│ - VPC        │        │ - Images     │
│ - EC2        │        └──────────────┘
│ - DynamoDB   │                │
│ - S3         │                │
│ - IAM        │                │
└──────────────┘                │
        │                       │
        └───────────┬───────────┘
                    ▼
            ┌──────────────┐
            │ EC2 Instance │
            │              │
            │ Docker       │
            │ + Nginx      │
            │ + SSL        │
            └──────────────┘
```

## Infrastructure Components

### AWS Resources (Managed by Terraform)

- **VPC** - Isolated network with public/private subnets
- **EC2** - Application server with Docker
- **DynamoDB** - 4 tables (movies, persons, attributes, encoder)
- **S3** - File storage bucket
- **IAM** - Roles and policies (no keys needed)
- **Security Groups** - Firewall rules (ports 80, 443, 22)

### Application Stack (On EC2)

- **Docker** - Container runtime
- **Docker Compose** - Multi-container orchestration
- **Nginx** - Reverse proxy with SSL
- **Go API** - Application container

## Deployment Workflow

### 1. Infrastructure Changes

```bash
# Edit infrastructure
vim infra/tofu/envs/dev/variables.tf

# Push changes
git push origin dev

# GitHub Actions automatically:
# - Plans changes (on PR)
# - Applies changes (on merge)
# - Outputs EC2 IP
```

### 2. Application Changes

```bash
# Edit code
vim internal/http/movie_handler.go

# Push changes
git push origin dev

# GitHub Actions automatically:
# - Builds Docker image
# - Pushes to Docker Hub

# Then deploy to EC2:
ssh ec2-user@<EC2_IP>
cd /opt/bi8s/compose
../scripts/deploy.sh
```

## Environment Variables

### GitHub Secrets (CI/CD)

Required in repository settings:

- `AWS_ACCESS_KEY_ID` - AWS credentials
- `AWS_SECRET_ACCESS_KEY` - AWS credentials
- `AWS_REGION` - AWS region
- `DOCKER_REGISTRY` - Docker Hub URL
- `DOCKER_USERNAME` - Docker Hub username
- `DOCKER_PASSWORD` - Docker Hub token

### EC2 Environment (Application)

Configured in `/opt/bi8s/compose/.env`:

- Auto-set by Terraform: AWS resources, region, table names
- Manual: Application secrets (JWT, API keys, etc.)

## Directory Structure on EC2

```
/opt/bi8s/
├── compose/
│   ├── docker-compose.yml
│   ├── .env (your secrets)
│   └── .env.example
├── nginx/
│   ├── conf.d/api.conf
│   └── ssl/live/
│       ├── cert.crt
│       └── cert.key
└── scripts/
    ├── deploy.sh
    ├── update-ip.sh
    ├── renew-ssl.sh
    └── backup-config.sh
```

## Common Tasks

### Deploy Application

```bash
ssh ec2-user@<EC2_IP>
cd /opt/bi8s/compose
../scripts/deploy.sh
```

### View Logs

```bash
docker-compose logs -f
```

### Update Configuration

```bash
vim /opt/bi8s/compose/.env
../scripts/deploy.sh
```

### Setup SSL Certificate

```bash
# From local machine
./infra/scripts/setup-ssl-letsencrypt.sh <EC2_IP> api.yourdomain.com
```

### Backup Configuration

```bash
/opt/bi8s/scripts/backup-config.sh
```

## Troubleshooting

### Check Service Status

```bash
cd /opt/bi8s/compose
docker-compose ps
```

### View Application Logs

```bash
docker-compose logs -f api
```

### View Nginx Logs

```bash
docker-compose logs -f nginx
```

### Restart Services

```bash
docker-compose restart
```

### Update IP After Restart

```bash
/opt/bi8s/scripts/update-ip.sh
docker-compose restart
```

## Security Notes

- IAM roles used (no AWS keys on EC2)
- SSL/TLS encryption (HTTPS)
- Security groups restrict access
- Secrets in `.env` (not in git)
- Docker containers isolated
- Nginx reverse proxy

## Monitoring

### Health Check

```bash
curl https://<EC2_IP>/v1/health
```

### Check Resources

```bash
# CPU/Memory
docker stats

# Disk space
df -h

# Network
netstat -tlnp
```

## Further Reading

- [Complete Deployment Guide](deployment-guide.md)
- [GitHub Actions Workflows](../.github/workflows/README.md)
- [Architecture Documentation](architecture.md)
- [DynamoDB Design](dynamodb.md)
