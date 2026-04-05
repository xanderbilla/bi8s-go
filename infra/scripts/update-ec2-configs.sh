#!/bin/bash
set -e

# Script to update configs on EC2 without recreating instance
# Usage: ./update-ec2-configs.sh <ec2-ip>

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <ec2-ip>"
    echo "Example: $0 54.123.45.67"
    exit 1
fi

EC2_IP=$1
REPO_URL="https://github.com/xanderbilla/bi8s-go.git"

echo "Updating EC2 Configurations"
echo "EC2 IP: ${EC2_IP}"
echo ""

echo ""
echo "Connecting to EC2 and updating configs..."
ssh -o StrictHostKeyChecking=no ec2-user@${EC2_IP} << 'EOF'
    set -e
    
    echo "Cloning latest repository..."
    cd /tmp
    rm -rf bi8s-go
    git clone https://github.com/xanderbilla/bi8s-go.git
    cd bi8s-go
    
    echo "Backing up current configs..."
    /opt/bi8s/scripts/backup-config.sh
    
    echo "Updating scripts..."
    sudo cp infra/scripts/*.sh /opt/bi8s/scripts/
    sudo chmod +x /opt/bi8s/scripts/*.sh
    
    echo "Updating docker-compose..."
    sudo cp infra/docker/docker-compose.yml /opt/bi8s/compose/
    
    echo "Updating nginx config..."
    sudo cp infra/docker/nginx.conf /opt/bi8s/nginx/conf.d/api.conf
    
    echo "Setting permissions..."
    sudo chown -R ec2-user:ec2-user /opt/bi8s
    
    echo "Restarting services..."
    cd /opt/bi8s/compose
    docker-compose restart nginx
    
    echo "Checking service status..."
    docker-compose ps
    
    echo "Cleaning up..."
    rm -rf /tmp/bi8s-go
    
    echo ""
    echo "Configs updated successfully!"
EOF

echo ""
echo "Update Complete!"
echo ""
echo "Updated:"
echo "  - Helper scripts"
echo "  - Docker Compose config"
echo "  - Nginx config"
echo ""
echo "Services restarted and running!"
