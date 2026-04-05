#!/bin/bash
set -e

# Script to build Docker image and deploy to EC2 instance
# Usage: ./infra/scripts/docker-deploy.sh <environment> <ec2-ip>

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <environment> <ec2-ip>"
    echo "Example: $0 dev 54.123.45.67"
    exit 1
fi

ENVIRONMENT=$1
EC2_IP=$2
PROJECT_NAME="bi8s"
IMAGE_NAME="${PROJECT_NAME}-api"
IMAGE_TAG="${ENVIRONMENT}-$(date +%Y%m%d-%H%M%S)"

echo "Building and Deploying Docker Image"
echo "Environment: ${ENVIRONMENT}"
echo "EC2 IP: ${EC2_IP}"
echo "Image: ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""

# Build Docker image
echo "Building Docker image..."
docker build -f infra/docker/Dockerfile.deploy -t ${IMAGE_NAME}:${IMAGE_TAG} .
docker tag ${IMAGE_NAME}:${IMAGE_TAG} ${IMAGE_NAME}:latest

# Save Docker image to tar file
echo "Saving Docker image..."
docker save ${IMAGE_NAME}:latest | gzip > ${IMAGE_NAME}.tar.gz

# Copy image to EC2
echo "Copying Docker image to EC2..."
scp -o StrictHostKeyChecking=no ${IMAGE_NAME}.tar.gz ec2-user@${EC2_IP}:/tmp/

# Copy docker-compose file
echo "Copying docker-compose file..."
scp -o StrictHostKeyChecking=no infra/docker/docker-compose.yml ec2-user@${EC2_IP}:/opt/${PROJECT_NAME}/

# Load image and restart containers on EC2
echo "Loading image and restarting containers..."
ssh -o StrictHostKeyChecking=no ec2-user@${EC2_IP} << EOF
    # Load Docker image
    docker load < /tmp/${IMAGE_NAME}.tar.gz
    rm /tmp/${IMAGE_NAME}.tar.gz
    
    # Navigate to app directory
    cd /opt/${PROJECT_NAME}
    
    # Stop existing containers
    docker-compose down || true
    
    # Start new containers
    docker-compose up -d
    
    # Show container status
    docker-compose ps
    
    # Show logs
    docker-compose logs --tail=50
EOF

# Clean up local files
rm -f ${IMAGE_NAME}.tar.gz

echo ""
echo "Docker Deployment Complete!"
echo ""
echo "Application is running at: http://${EC2_IP}:8080"
echo "Health check: http://${EC2_IP}:8080/v1/health"
echo ""
echo "To view logs: ssh ec2-user@${EC2_IP} 'cd /opt/${PROJECT_NAME} && docker-compose logs -f'"
