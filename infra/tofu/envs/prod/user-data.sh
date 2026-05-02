#!/bin/bash
set -e

# Log output to file
exec > >(tee /var/log/user-data.log)
exec 2>&1

echo "Starting EC2 User Data Script"
echo ""

# Update system
echo "Updating system packages..."
dnf update -y

# Install required packages
echo "Installing required packages..."
dnf install -y git wget tar curl unzip openssl jq

# Install Docker
echo "Installing Docker..."
dnf install -y docker

# Start and enable Docker service
echo "Starting Docker service..."
systemctl start docker
systemctl enable docker

# Add ec2-user to docker group
echo "Adding ec2-user to docker group..."
usermod -aG docker ec2-user

# Install Docker Compose
echo "Installing Docker Compose..."
DOCKER_COMPOSE_VERSION="2.24.5"
curl -L "https://github.com/docker/compose/releases/download/v$${DOCKER_COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose
ln -sf /usr/local/bin/docker-compose /usr/bin/docker-compose

# Verify installations
echo "Verifying installations..."
docker --version
docker-compose --version

# Install Go 1.25
echo "Installing Go..."
GO_VERSION="1.25.0"
wget https://go.dev/dl/go$${GO_VERSION}.linux-amd64.tar.gz
rm -rf /usr/local/go
tar -C /usr/local -xzf go$${GO_VERSION}.linux-amd64.tar.gz
rm go$${GO_VERSION}.linux-amd64.tar.gz

# Set up Go environment
echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile.d/go.sh
echo 'export GOPATH=/home/ec2-user/go' >> /etc/profile.d/go.sh

# Verify Go installation
/usr/local/go/bin/go version

# Create proper directory structure
echo "Creating application directory structure..."
mkdir -p /opt/${project_name}/{compose,nginx/{conf.d,ssl/{live,archive,renewal},certbot/www},scripts}
cd /opt/${project_name}

# Get current public IP
echo "Fetching EC2 public IP..."
PUBLIC_IP=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)
echo "Public IP: $PUBLIC_IP"

# Generate self-signed SSL certificate with IP
echo "Generating self-signed SSL certificate..."
openssl req -x509 -nodes -days 365 \
  -newkey rsa:2048 \
  -keyout /opt/${project_name}/nginx/ssl/live/cert.key \
  -out /opt/${project_name}/nginx/ssl/live/cert.crt \
  -subj "/C=US/ST=State/L=City/O=${project_name}/CN=$PUBLIC_IP"

# Set proper permissions for certificates
chmod 644 /opt/${project_name}/nginx/ssl/live/cert.crt
chmod 600 /opt/${project_name}/nginx/ssl/live/cert.key

# Create certbot webroot
mkdir -p /opt/${project_name}/nginx/certbot/www

# Create nginx config
cat > /opt/${project_name}/nginx/conf.d/api.conf <<'NGINXCONF'
upstream api_backend {
    server api:8080;
}

# HTTP Server - Redirect to HTTPS
server {
    listen 80;
    server_name _;

    # Certbot challenge
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    # Health check (no redirect)
    location /health {
        access_log off;
        return 200 "healthy\n";
        add_header Content-Type text/plain;
    }

    # Redirect to HTTPS
    location / {
        return 301 https://$host$request_uri;
    }
}

# HTTPS Server
server {
    listen 443 ssl;
    http2 on;
    server_name _;

    # SSL Configuration
    ssl_certificate /etc/nginx/ssl/live/cert.crt;
    ssl_certificate_key /etc/nginx/ssl/live/cert.key;
    
    # SSL Security
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # Security Headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Client body size
    client_max_body_size 100M;

    # API Proxy
    location / {
        proxy_pass http://api_backend;
        proxy_http_version 1.1;
        
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    location /health {
        access_log off;
        return 200 "healthy\n";
        add_header Content-Type text/plain;
    }
}
NGINXCONF

# Set up environment variables
echo "Setting up environment variables..."
cat > /etc/profile.d/${project_name}.sh <<EOF
export APP_ENV="${environment}"
export AWS_REGION="${aws_region}"
export DYNAMODB_MOVIE_TABLE="${dynamodb_movie_table}"
export DYNAMODB_PERSON_TABLE="${dynamodb_person_table}"
export DYNAMODB_ATTRIBUTE_TABLE="${dynamodb_attribute_table}"
export DYNAMODB_ENCODER_TABLE="${dynamodb_encoder_table}"
export S3_BUCKET="${s3_bucket}"
export CORS_ALLOWED_ORIGINS="*"
export CORS_ALLOW_PRIVATE_NETWORK="true"
export PUBLIC_IP="$PUBLIC_IP"
EOF

# Create .env file for Docker Compose
cat > /opt/${project_name}/compose/.env <<EOF
PROJECT_NAME=${project_name}
IMAGE_NAME=xanderbilla/my-project:latest
APP_ENV=${environment}
AWS_REGION=${aws_region}
DYNAMODB_MOVIE_TABLE=${dynamodb_movie_table}
DYNAMODB_PERSON_TABLE=${dynamodb_person_table}
DYNAMODB_ATTRIBUTE_TABLE=${dynamodb_attribute_table}
DYNAMODB_ENCODER_TABLE=${dynamodb_encoder_table}
S3_BUCKET=${s3_bucket}
CORS_ALLOWED_ORIGINS=*
CORS_ALLOW_PRIVATE_NETWORK=true
PUBLIC_IP=$PUBLIC_IP
EOF

# Create .env.example with all required variables
cat > /opt/${project_name}/compose/.env.example <<'ENVEXAMPLE'
# Project Configuration
PROJECT_NAME=bi8s
IMAGE_NAME=xanderbilla/my-project:latest

# Environment
APP_ENV=dev

# AWS Configuration
AWS_REGION=us-east-1

# DynamoDB Tables
DYNAMODB_MOVIE_TABLE=bi8s-content-table-dev
DYNAMODB_PERSON_TABLE=bi8s-person-table-dev
DYNAMODB_ATTRIBUTE_TABLE=bi8s-attributes-table-dev
DYNAMODB_ENCODER_TABLE=bi8s-video-table-dev

# S3 Storage
S3_BUCKET=bi8s-storage-dev

# CORS Configuration
CORS_ALLOWED_ORIGINS=*
CORS_ALLOW_PRIVATE_NETWORK=true

# Public IP (auto-detected, don't change)
PUBLIC_IP=auto-detected

# ============================================
# ADD YOUR CUSTOM ENV VARIABLES BELOW
# ============================================

# Example: Database credentials (if needed)
# DB_HOST=localhost
# DB_PORT=5432
# DB_USER=myuser
# DB_PASSWORD=mypassword

# Example: API Keys (if needed)
# STRIPE_API_KEY=sk_test_xxxxx
# SENDGRID_API_KEY=SG.xxxxx

# Example: JWT Secret
# JWT_SECRET=your-secret-key-here

# Example: Other services
# REDIS_URL=redis://localhost:6379
# ELASTICSEARCH_URL=http://localhost:9200
ENVEXAMPLE

# Create docker-compose.yml
cat > /opt/${project_name}/compose/docker-compose.yml <<'DOCKERCOMPOSE'
version: '3.8'

services:
  api:
    image: $${IMAGE_NAME:-xanderbilla/my-project:latest}
    container_name: $${PROJECT_NAME:-bi8s}-api
    restart: unless-stopped
    expose:
      - "8080"
    environment:
      - APP_ENV=$${APP_ENV}
      - AWS_REGION=$${AWS_REGION}
      - DYNAMODB_MOVIE_TABLE=$${DYNAMODB_MOVIE_TABLE}
      - DYNAMODB_PERSON_TABLE=$${DYNAMODB_PERSON_TABLE}
      - DYNAMODB_ATTRIBUTE_TABLE=$${DYNAMODB_ATTRIBUTE_TABLE}
      - DYNAMODB_ENCODER_TABLE=$${DYNAMODB_ENCODER_TABLE}
      - S3_BUCKET=$${S3_BUCKET}
      - CORS_ALLOWED_ORIGINS=$${CORS_ALLOWED_ORIGINS}
      - CORS_ALLOW_PRIVATE_NETWORK=$${CORS_ALLOW_PRIVATE_NETWORK}
    networks:
      - app-network
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/v1/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  nginx:
    image: nginx:alpine
    container_name: $${PROJECT_NAME:-bi8s}-nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ../nginx/conf.d:/etc/nginx/conf.d:ro
      - ../nginx/ssl:/etc/nginx/ssl:ro
      - ../nginx/certbot/www:/var/www/certbot:ro
    depends_on:
      - api
    networks:
      - app-network
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://127.0.0.1/health"]
      interval: 30s
      timeout: 10s
      retries: 3

networks:
  app-network:
    driver: bridge
DOCKERCOMPOSE

# Create systemd service for Docker Compose
cat > /etc/systemd/system/${project_name}-docker.service <<EOF
[Unit]
Description=${project_name} Docker Compose Service
Requires=docker.service
After=docker.service network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/opt/${project_name}/compose
ExecStartPre=/opt/${project_name}/scripts/update-ip.sh
ExecStart=/usr/local/bin/docker-compose up -d
ExecStop=/usr/local/bin/docker-compose down
TimeoutStartSec=300
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Set permissions
echo "Setting permissions..."
chown -R ec2-user:ec2-user /opt/${project_name}
chmod 644 /opt/${project_name}/.env

# Reload systemd
systemctl daemon-reload

# Enable service
systemctl enable ${project_name}-docker.service

# Install AWS CLI v2
echo "Installing AWS CLI v2..."
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip -q awscliv2.zip
./aws/install
rm -rf aws awscliv2.zip

# Verify AWS CLI and IAM role
echo "Verifying AWS CLI and IAM role..."
aws --version
aws sts get-caller-identity || echo "IAM role will be available after instance is fully initialized"

# Create IP update script (runs on every restart)
cat > /opt/${project_name}/scripts/update-ip.sh <<'SCRIPT'
#!/bin/bash
# Auto-update public IP on restart

set -e

echo "Updating public IP..."
NEW_IP=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)
OLD_IP=$(grep "^PUBLIC_IP=" /opt/${project_name}/compose/.env | cut -d'=' -f2)

if [ "$NEW_IP" != "$OLD_IP" ]; then
    echo "IP changed: $OLD_IP -> $NEW_IP"
    
    # Update .env file
    sed -i "s/^PUBLIC_IP=.*/PUBLIC_IP=$NEW_IP/" /opt/${project_name}/compose/.env
    
    # Update environment profile
    sed -i "s/^export PUBLIC_IP=.*/export PUBLIC_IP=\"$NEW_IP\"/" /etc/profile.d/${project_name}.sh
    
    # Regenerate self-signed certificate with new IP
    openssl req -x509 -nodes -days 365 \
      -newkey rsa:2048 \
      -keyout /opt/${project_name}/nginx/ssl/live/cert.key \
      -out /opt/${project_name}/nginx/ssl/live/cert.crt \
      -subj "/C=US/ST=State/L=City/O=${project_name}/CN=$NEW_IP" 2>/dev/null
    
    chmod 644 /opt/${project_name}/nginx/ssl/live/cert.crt
    chmod 600 /opt/${project_name}/nginx/ssl/live/cert.key
    
    echo "IP and SSL certificate updated successfully!"
else
    echo "IP unchanged: $NEW_IP"
fi
SCRIPT

chmod +x /opt/${project_name}/scripts/update-ip.sh

# Create helper script for SSL certificate renewal with Let's Encrypt
cat > /opt/${project_name}/scripts/renew-ssl.sh <<'SCRIPT'
#!/bin/bash
# Script to renew SSL certificate with Let's Encrypt
# Usage: ./renew-ssl.sh yourdomain.com

DOMAIN=$1
if [ -z "$DOMAIN" ]; then
    echo "Usage: $0 <domain>"
    exit 1
fi

# Install certbot if not present
if ! command -v certbot &> /dev/null; then
    echo "Installing certbot..."
    dnf install -y certbot
fi

# Stop nginx container
docker-compose stop nginx

# Get certificate using webroot
certbot certonly --webroot \
    -w /opt/${project_name}/nginx/certbot/www \
    -d $DOMAIN \
    --non-interactive \
    --agree-tos \
    --email admin@$DOMAIN

# Copy certificates
cp /etc/letsencrypt/live/$DOMAIN/fullchain.pem /opt/${project_name}/nginx/ssl/live/cert.crt
cp /etc/letsencrypt/live/$DOMAIN/privkey.pem /opt/${project_name}/nginx/ssl/live/cert.key

# Backup to archive
mkdir -p /opt/${project_name}/nginx/ssl/archive/$DOMAIN
cp /etc/letsencrypt/live/$DOMAIN/* /opt/${project_name}/nginx/ssl/archive/$DOMAIN/

# Set permissions
chmod 644 /opt/${project_name}/nginx/ssl/live/cert.crt
chmod 600 /opt/${project_name}/nginx/ssl/live/cert.key

# Restart nginx container
cd /opt/${project_name}/compose
docker-compose restart nginx

echo "SSL certificate renewed successfully!"
SCRIPT

chmod +x /opt/${project_name}/scripts/renew-ssl.sh

# Create backup script
cat > /opt/${project_name}/scripts/backup-config.sh <<'SCRIPT'
#!/bin/bash
# Backup configuration files

BACKUP_DIR="/opt/${project_name}/backups/$(date +%Y%m%d_%H%M%S)"
mkdir -p $BACKUP_DIR

echo "Creating backup in $BACKUP_DIR..."

# Backup compose files
cp -r /opt/${project_name}/compose $BACKUP_DIR/

# Backup nginx config
cp -r /opt/${project_name}/nginx/conf.d $BACKUP_DIR/

# Backup SSL certificates
cp -r /opt/${project_name}/nginx/ssl $BACKUP_DIR/

echo "Backup completed!"
ls -lh $BACKUP_DIR
SCRIPT

chmod +x /opt/${project_name}/scripts/backup-config.sh

# Create deployment helper script
cat > /opt/${project_name}/scripts/deploy.sh <<'SCRIPT'
#!/bin/bash
# Deploy/Redeploy application
set -e

cd /opt/${project_name}/compose

echo "Deploying Application"
echo ""

# Check if .env exists
if [ ! -f ".env" ]; then
    echo "❌ Error: .env file not found!"
    echo ""
    echo "Please create .env file first:"
    echo "  1. cp .env.example .env"
    echo "  2. vim .env (edit with your values)"
    echo "  3. Run this script again"
    echo ""
    exit 1
fi

echo "✅ .env file found"

# Update IP address
echo ""
echo "Updating IP address..."
/opt/${project_name}/scripts/update-ip.sh

# Pull latest image
echo ""
echo "Pulling latest Docker image..."
docker-compose pull

# Stop old containers
echo ""
echo "Stopping old containers..."
docker-compose down

# Start new containers
echo ""
echo "Starting containers..."
docker-compose up -d

# Wait for services
echo ""
echo "Waiting for services to be healthy..."
sleep 15

# Check status
echo ""
echo "Service status:"
docker-compose ps

# Show logs
echo ""
echo "Recent logs:"
docker-compose logs --tail=30

# Get public IP
PUBLIC_IP=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)

echo ""
echo "Deployment Complete!"
echo ""
echo "Application is running!"
echo ""
echo "Access your API:"
echo "  HTTP:  http://$PUBLIC_IP"
echo "  HTTPS: https://$PUBLIC_IP"
echo ""
echo "Health check:"
echo "  curl https://$PUBLIC_IP/v1/health"
echo ""
echo "View logs:"
echo "  docker-compose logs -f"
echo ""
SCRIPT

chmod +x /opt/${project_name}/scripts/deploy.sh

echo "User Data Script Completed Successfully!"
echo ""
echo "Docker: $(docker --version)"
echo "Docker Compose: $(docker-compose --version)"
echo "Go: $(/usr/local/go/bin/go version)"
echo "AWS CLI: $(aws --version)"
echo ""
echo "Application directory: /opt/${project_name}"
echo "Directory structure:"
echo "  /opt/${project_name}/compose/          - Docker Compose files"
echo "  /opt/${project_name}/nginx/conf.d/     - Nginx configuration"
echo "  /opt/${project_name}/nginx/ssl/        - SSL certificates"
echo "  /opt/${project_name}/scripts/          - Helper scripts"
echo ""
echo "Current Public IP: $PUBLIC_IP"
echo ""
echo "Helper scripts:"
echo "  /opt/${project_name}/scripts/deploy.sh              - Deploy/update application"
echo "  /opt/${project_name}/scripts/update-ip.sh           - Update IP (auto-runs on restart)"
echo "  /opt/${project_name}/scripts/renew-ssl.sh <domain>  - Setup Let's Encrypt SSL"
echo "  /opt/${project_name}/scripts/backup-config.sh       - Backup configuration"
echo ""
echo "To deploy application:"
echo "  cd /opt/${project_name}/compose"
echo "  docker-compose up -d"
echo ""
echo "To renew SSL with Let's Encrypt:"
echo "  /opt/${project_name}/scripts/renew-ssl.sh yourdomain.com"
echo ""
echo "=========================================="

