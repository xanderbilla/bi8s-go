#!/bin/bash
set -e

# Script to deploy infrastructure using OpenTofu/Terraform
# Usage: ./scripts/deploy.sh <environment> [plan|apply|destroy]

# Check arguments
if [ "$#" -lt 1 ]; then
    echo "Usage: $0 <environment> [plan|apply|destroy]"
    echo "Example: $0 dev plan"
    echo "Example: $0 prod apply"
    exit 1
fi

ENVIRONMENT=$1
ACTION=${2:-plan}

# Validate environment
if [[ ! "$ENVIRONMENT" =~ ^(dev|prod)$ ]]; then
    echo "Error: Environment must be dev or prod"
    exit 1
fi

# Validate action
if [[ ! "$ACTION" =~ ^(plan|apply|destroy)$ ]]; then
    echo "Error: Action must be plan, apply, or destroy"
    exit 1
fi

# Change to environment directory
cd "$(dirname "$0")/../infra/tofu/envs/${ENVIRONMENT}" || exit 1

BACKEND_CONFIG="backend-${ENVIRONMENT}.hcl"

echo "Deploying Infrastructure"
echo "Environment: ${ENVIRONMENT}"
echo "Action: ${ACTION}"
echo "Backend Config: ${BACKEND_CONFIG}"
echo "Working Directory: $(pwd)"
echo ""

# Check if backend config exists
if [ ! -f "${BACKEND_CONFIG}" ]; then
    echo "Error: Backend config ${BACKEND_CONFIG} not found"
    echo "Please run: ./scripts/init-backend.sh bi8s ${ENVIRONMENT} us-east-1"
    exit 1
fi

# Check if tofu or terraform is available
if command -v tofu &> /dev/null; then
    TF_CMD="tofu"
elif command -v terraform &> /dev/null; then
    TF_CMD="terraform"
else
    echo "Error: Neither tofu nor terraform is installed"
    exit 1
fi

echo "Using: ${TF_CMD}"
echo ""

# Initialize if needed
if [ ! -d ".terraform" ]; then
    echo ""
    echo "Initializing ${TF_CMD}..."
    ${TF_CMD} init -backend-config="${BACKEND_CONFIG}"
fi

# Execute action
echo ""
echo "Executing ${ACTION}..."
case $ACTION in
    plan)
        ${TF_CMD} plan -out="${ENVIRONMENT}.tfplan"
        echo ""
        echo "Plan saved to ${ENVIRONMENT}.tfplan"
        echo "To apply: ${TF_CMD} apply ${ENVIRONMENT}.tfplan"
        ;;
    apply)
        if [ -f "${ENVIRONMENT}.tfplan" ]; then
            echo "Applying saved plan..."
            ${TF_CMD} apply "${ENVIRONMENT}.tfplan"
            rm -f "${ENVIRONMENT}.tfplan"
        else
            ${TF_CMD} apply -auto-approve
        fi
        # Remove any stale /etc/hosts overrides for project domains so DNS resolves correctly
        if grep -q "xanderbilla" /etc/hosts 2>/dev/null; then
            echo ""
            echo "Removing stale /etc/hosts entries for xanderbilla.com domains..."
            sudo sed -i '' '/xanderbilla/d' /etc/hosts
            echo "Done."
        fi
        ;;
    destroy)
        echo "WARNING: This will destroy all resources in ${ENVIRONMENT} environment!"
        read -p "Are you sure? (yes/no): " confirm
        if [ "$confirm" = "yes" ]; then
            ${TF_CMD} destroy -auto-approve
        else
            echo "Destroy cancelled."
            exit 0
        fi
        ;;
esac

echo ""
echo "Deployment Complete!"
