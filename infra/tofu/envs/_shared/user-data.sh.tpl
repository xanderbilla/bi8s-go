#!/bin/bash
set -e

# Log output to file
exec > >(tee /var/log/user-data.log)
exec 2>&1

echo "Starting EC2 User Data Script"
echo ""

export DEBIAN_FRONTEND=noninteractive

# Update system
echo "Updating system packages..."
apt-get update -y
apt-get upgrade -y

# Install required packages
echo "Installing required packages..."
apt-get install -y git wget tar curl unzip openssl jq ca-certificates gnupg lsb-release

# Install Docker CE (official Docker repo)
echo "Installing Docker CE..."
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list
apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io

# Start and enable Docker service
echo "Starting Docker service..."
systemctl start docker
systemctl enable docker

# Add ubuntu to docker group
echo "Adding ubuntu to docker group..."
usermod -aG docker ubuntu

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
echo 'export GOPATH=/home/ubuntu/go' >> /etc/profile.d/go.sh

# Verify Go installation
/usr/local/go/bin/go version

# Create proper directory structure
echo "Creating application directory structure..."
mkdir -p /opt/${project_name}/{compose,scripts,prometheus-data}
cd /opt/${project_name}

# Mount Prometheus EBS volume
echo "Setting up Prometheus EBS volume..."
PROM_DEVICE="${prometheus_device}"
if [ -b "$PROM_DEVICE" ]; then
  # Format only if no filesystem exists on the device
  if ! blkid "$PROM_DEVICE"; then
    echo "Formatting Prometheus EBS volume..."
    mkfs.ext4 -F "$PROM_DEVICE"
  fi
  mount "$PROM_DEVICE" /opt/${project_name}/prometheus-data
  # Persist across reboots
  PROM_UUID=$(blkid -s UUID -o value "$PROM_DEVICE")
  echo "UUID=$PROM_UUID /opt/${project_name}/prometheus-data ext4 defaults,nofail 0 2" >> /etc/fstab
  echo "Prometheus volume mounted at /opt/${project_name}/prometheus-data"
else
  echo "WARNING: Prometheus device $PROM_DEVICE not found, skipping mount."
fi

# Get current public IP (IMDSv2)
echo "Fetching EC2 public IP..."
IMDS_TOKEN=$(curl -s -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
PUBLIC_IP=$(curl -s -H "X-aws-ec2-metadata-token: $IMDS_TOKEN" http://169.254.169.254/latest/meta-data/public-ipv4)
echo "Public IP: $PUBLIC_IP"

# Set up environment variables
echo "Setting up environment variables..."
cat > /etc/profile.d/${project_name}.sh <<EOF
export APP_ENV="${environment}"
export AWS_REGION="${aws_region}"
export DYNAMODB_CONTENT_TABLE="${dynamodb_movie_table}"
export DYNAMODB_PERSON_TABLE="${dynamodb_person_table}"
export DYNAMODB_ATTRIBUTE_TABLE="${dynamodb_attribute_table}"
export DYNAMODB_ATTRIBUTE_NAME_INDEX="${dynamodb_attribute_name_index}"
export DYNAMODB_CONTENT_CAST_TABLE="${dynamodb_content_cast_table}"
export DYNAMODB_CONTENT_ATTRIBUTE_TABLE="${dynamodb_content_attribute_table}"
export DYNAMODB_CONTENT_VISIBILITY_CREATED_AT_INDEX="visibility-createdAt-index"
export DYNAMODB_CONTENT_VISIBILITY_CONTENT_TYPE_INDEX="visibility-contentType-index"
export S3_BUCKET="${s3_bucket}"
export CORS_ALLOWED_ORIGINS="%{ if domain_name != "" }https://${domain_name},http://${domain_name},%{ endif }%{ if grafana_domain_name != "" }https://${grafana_domain_name},%{ endif }%{ if amplify_url != "" }${amplify_url},%{ endif }http://localhost:3000,http://localhost:8080,http://$PUBLIC_IP"
export CORS_ALLOW_PRIVATE_NETWORK="true"
export PUBLIC_IP="$PUBLIC_IP"
EOF

# Create .env file for Docker Compose (IAM instance profile provides AWS credentials via IMDS).
cat > /opt/${project_name}/compose/.env <<EOF
PROJECT_NAME=${project_name}
IMAGE_NAME=${image_name}
APP_ENV=${environment}
PORT=:8080
LOG_LEVEL=info
LOG_ADD_SOURCE=false
AWS_REGION=${aws_region}
DYNAMODB_CONTENT_TABLE=${dynamodb_movie_table}
DYNAMODB_PERSON_TABLE=${dynamodb_person_table}
DYNAMODB_ATTRIBUTE_TABLE=${dynamodb_attribute_table}
DYNAMODB_ATTRIBUTE_NAME_INDEX=${dynamodb_attribute_name_index}
DYNAMODB_CONTENT_CAST_TABLE=${dynamodb_content_cast_table}
DYNAMODB_CONTENT_ATTRIBUTE_TABLE=${dynamodb_content_attribute_table}
DYNAMODB_CONTENT_VISIBILITY_CREATED_AT_INDEX=visibility-createdAt-index
DYNAMODB_CONTENT_VISIBILITY_CONTENT_TYPE_INDEX=visibility-contentType-index
DYNAMODB_MAX_SCAN_PAGES=1000
CTX_DB_TIMEOUT_MS=30000
S3_BUCKET=${s3_bucket}
CORS_ALLOWED_ORIGINS=%{ if domain_name != "" }https://${domain_name},http://${domain_name},%{ endif }%{ if grafana_domain_name != "" }https://${grafana_domain_name},%{ endif }%{ if amplify_url != "" }${amplify_url},%{ endif }http://localhost:3000,http://localhost:8080,http://$${PUBLIC_IP}
CORS_ALLOW_PRIVATE_NETWORK=true
TRUSTED_PROXIES=
PUBLIC_IP=$${PUBLIC_IP}
OTEL_SERVICE_NAME=${project_name}-api
OTEL_ENABLED=true
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
OTEL_EXPORTER_OTLP_INSECURE=true
OTEL_TRACES_ENABLED=true
OTEL_METRICS_ENABLED=true
OTEL_TRACES_SAMPLER_ARG=1.0
OTEL_METRIC_EXPORT_INTERVAL_SECONDS=15
OTEL_SHUTDOWN_TIMEOUT_SECONDS=5
BUILD_VERSION=${environment}
PROMETHEUS_RETENTION=168h
GRAFANA_ADMIN_USER=${grafana_admin_user}
GRAFANA_ADMIN_PASSWORD=${grafana_admin_password}
GRAFANA_ROOT_URL=%{ if grafana_domain_name != "" }https://${grafana_domain_name}/${project_name}%{ else }http://$${PUBLIC_IP}/${project_name}%{ endif }
GF_SERVER_SERVE_FROM_SUB_PATH=true
STORAGE_BASE_URL=%{ if storage_domain_name != "" }https://${storage_domain_name}/${project_name}%{ else }http://$${PUBLIC_IP}/${project_name}%{ endif }
EOF

# Clone application repo (single source of truth for compose + observability configs).
echo "Cloning application repo (${repo_url}@${repo_branch})..."
rm -rf /opt/${project_name}/repo
git clone --depth 1 --branch "${repo_branch}" "${repo_url}" /opt/${project_name}/repo

# Symlink the canonical compose + observability configs into /opt/${project_name}/compose/
ln -sf /opt/${project_name}/repo/infra/docker/docker-compose.yml /opt/${project_name}/compose/docker-compose.yml
ln -sfn /opt/${project_name}/repo/observability /opt/${project_name}/compose/observability

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

# Set permissions (do not dereference symlinks; keep prometheus-data writable by root container).
echo "Setting permissions..."
chown -RH ubuntu:ubuntu /opt/${project_name}/compose /opt/${project_name}/scripts
chown -R ubuntu:ubuntu /opt/${project_name}/repo
chmod 600 /opt/${project_name}/compose/.env

# Create seed-on-boot script (runs seed.sh after Docker service is up)
mkdir -p /opt/${project_name}/logs
cat > /opt/${project_name}/scripts/seed-on-boot.sh <<'SEEDSCRIPT'
#!/bin/bash
set -euo pipefail

LOG_FILE="/opt/${project_name}/logs/seed.log"
REPO_DIR="/opt/${project_name}/repo"
SEED_SCRIPT="$REPO_DIR/scripts/seed.sh"

echo "[seed-on-boot] $(date) Starting..." >> "$LOG_FILE"

# Wait for systemd service to be active
for i in $(seq 1 30); do
  if systemctl is-active --quiet ${project_name}-docker.service; then
    echo "[seed-on-boot] $(date) Docker service active." >> "$LOG_FILE"
    break
  fi
  echo "[seed-on-boot] $(date) Waiting for docker service ($i/30)..." >> "$LOG_FILE"
  sleep 10
done

# Run seed script
if [ -f "$SEED_SCRIPT" ]; then
  bash "$SEED_SCRIPT" >> "$LOG_FILE" 2>&1
  echo "[seed-on-boot] $(date) Seed finished (exit $?)." >> "$LOG_FILE"
else
  echo "[seed-on-boot] $(date) ERROR: seed.sh not found at $SEED_SCRIPT" >> "$LOG_FILE"
fi
SEEDSCRIPT

chmod +x /opt/${project_name}/scripts/seed-on-boot.sh
nohup /opt/${project_name}/scripts/seed-on-boot.sh > /opt/${project_name}/logs/seed.log 2>&1 &

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
IMDS_TOKEN=$(curl -s -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
NEW_IP=$(curl -s -H "X-aws-ec2-metadata-token: $IMDS_TOKEN" http://169.254.169.254/latest/meta-data/public-ipv4)
OLD_IP=$(grep "^PUBLIC_IP=" /opt/${project_name}/compose/.env | cut -d'=' -f2)

if [ "$NEW_IP" != "$OLD_IP" ]; then
    echo "IP changed: $OLD_IP -> $NEW_IP"
    
    # Update .env file
    sed -i "s/^PUBLIC_IP=.*/PUBLIC_IP=$NEW_IP/" /opt/${project_name}/compose/.env
    
    # Update environment profile
    sed -i "s/^export PUBLIC_IP=.*/export PUBLIC_IP=\"$NEW_IP\"/" /etc/profile.d/${project_name}.sh
    
    echo "IP updated successfully!"
else
    echo "IP unchanged: $NEW_IP"
fi
SCRIPT

chmod +x /opt/${project_name}/scripts/update-ip.sh

# Reload systemd and start service (after all scripts are in place)
systemctl daemon-reload
systemctl enable ${project_name}-docker.service

# Log in to ECR so Docker can pull the private image
echo "Logging in to Amazon ECR..."
aws ecr get-login-password --region ${aws_region} | \
  docker login --username AWS --password-stdin ${ecr_registry}

systemctl start ${project_name}-docker.service

# Create backup script
cat > /opt/${project_name}/scripts/backup-config.sh <<'SCRIPT'
#!/bin/bash
# Backup configuration files

BACKUP_DIR="/opt/${project_name}/backups/$(date +%Y%m%d_%H%M%S)"
mkdir -p $BACKUP_DIR

echo "Creating backup in $BACKUP_DIR..."

# Backup compose files
cp -r /opt/${project_name}/compose $BACKUP_DIR/

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
    echo "Error: .env file not found!"
    echo ""
    echo "Please create .env file first:"
    echo "  1. cp .env.example .env"
    echo "  2. vim .env (edit with your values)"
    echo "  3. Run this script again"
    echo ""
    exit 1
fi

echo ".env file found"

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

# Get public IP (IMDSv2)
IMDS_TOKEN=$(curl -s -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
PUBLIC_IP=$(curl -s -H "X-aws-ec2-metadata-token: $IMDS_TOKEN" http://169.254.169.254/latest/meta-data/public-ipv4)

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
echo "  /opt/${project_name}/scripts/          - Helper scripts"
echo ""
echo "Current Public IP: $PUBLIC_IP"
echo ""
echo "Helper scripts:"
echo "  /opt/${project_name}/scripts/deploy.sh              - Deploy/update application"
echo "  /opt/${project_name}/scripts/update-ip.sh           - Update IP (auto-runs on restart)"
echo "  /opt/${project_name}/scripts/backup-config.sh       - Backup configuration"
echo ""
echo "To deploy application:"
echo "  cd /opt/${project_name}/compose"
echo "  docker-compose up -d"
echo ""
echo "=========================================="

