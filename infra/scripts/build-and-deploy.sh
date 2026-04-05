#!/bin/bash
set -e

# Script to build and deploy application to EC2 instance
# Usage: ./infra/scripts/build-and-deploy.sh <environment> <ec2-ip>

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <environment> <ec2-ip>"
    echo "Example: $0 dev 54.123.45.67"
    exit 1
fi

ENVIRONMENT=$1
EC2_IP=$2
PROJECT_NAME="bi8s"

echo "Building and Deploying Application"
echo "Environment: ${ENVIRONMENT}"
echo "EC2 IP: ${EC2_IP}"
echo ""

# Build the Go binary
echo "Building Go binary..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o api ./cmd/api

# Copy binary to EC2
echo "Copying binary to EC2..."
scp -o StrictHostKeyChecking=no api ec2-user@${EC2_IP}:/opt/${PROJECT_NAME}/

# Restart the service
echo "Restarting service on EC2..."
ssh -o StrictHostKeyChecking=no ec2-user@${EC2_IP} "sudo systemctl restart ${PROJECT_NAME}.service"

# Check service status
echo "Checking service status..."
ssh -o StrictHostKeyChecking=no ec2-user@${EC2_IP} "sudo systemctl status ${PROJECT_NAME}.service"

# Clean up local binary
rm -f api

echo ""
echo "Deployment Complete!"
echo ""
echo "Application is running at: http://${EC2_IP}:8080"
echo "Health check: http://${EC2_IP}:8080/v1/health"
