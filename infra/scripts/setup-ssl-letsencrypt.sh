#!/bin/bash
set -e

# Script to setup Let's Encrypt SSL on EC2
# Usage: ./setup-ssl-letsencrypt.sh <ec2-ip> <domain>

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <ec2-ip> <domain>"
    echo "Example: $0 54.123.45.67 api.yourdomain.com"
    exit 1
fi

EC2_IP=$1
DOMAIN=$2

echo "Setting up Let's Encrypt SSL"
echo "EC2 IP: ${EC2_IP}"
echo "Domain: ${DOMAIN}"
echo ""

echo ""
echo "Step 1: Verify DNS is pointing to EC2"
echo "----------------------------------------"
RESOLVED_IP=$(dig +short ${DOMAIN} | tail -n1)
if [ "$RESOLVED_IP" != "$EC2_IP" ]; then
    echo "WARNING: DNS not pointing to EC2 IP!"
    echo "Expected: ${EC2_IP}"
    echo "Got: ${RESOLVED_IP}"
    echo ""
    echo "Please update your DNS A record:"
    echo "  ${DOMAIN} → ${EC2_IP}"
    echo ""
    read -p "Continue anyway? (yes/no): " confirm
    if [ "$confirm" != "yes" ]; then
        exit 1
    fi
fi

echo ""
echo "Step 2: Running SSL setup on EC2"
echo "----------------------------------------"
ssh -o StrictHostKeyChecking=no ec2-user@${EC2_IP} << EOF
    cd /opt/bi8s
    
    # Run the SSL renewal script
    sudo /opt/bi8s/renew-ssl.sh ${DOMAIN}
    
    echo ""
    echo "SSL certificate installed successfully!"
    echo ""
    echo "Restarting services..."
    docker-compose restart nginx
    
    echo ""
    echo "Service status:"
    docker-compose ps
EOF

echo ""
echo "SSL Setup Complete!"
echo ""
echo "Your API is now available at:"
echo "  https://${DOMAIN}"
echo ""
echo "Health check:"
echo "  https://${DOMAIN}/v1/health"
echo ""
echo "Certificate will auto-renew before expiry."
