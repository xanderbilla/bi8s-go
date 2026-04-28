.PHONY: help init-backend create-infra update-infra destroy-infra build-image push-image deploy-app clean test lint format build run

# Variables
PROJECT_NAME ?= $(shell echo $$PROJECT_NAME)
AWS_REGION ?= $(shell echo $$AWS_REGION)
DOCKER_IMAGE ?= $(shell echo $$DOCKER_IMAGE)
ENV ?= dev

# Colors
CYAN := \033[0;36m
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
MAGENTA := \033[0;35m
BOLD := \033[1m
RESET := \033[0m

# Check if variables are set
check-env:
	@if [ -z "$(PROJECT_NAME)" ]; then \
		printf "$(RED)Error:$(RESET) PROJECT_NAME not set\n"; \
		printf "$(YELLOW)Set it:$(RESET) export PROJECT_NAME=bi8s\n"; \
		exit 1; \
	fi
	@if [ -z "$(AWS_REGION)" ]; then \
		printf "$(RED)Error:$(RESET) AWS_REGION not set\n"; \
		printf "$(YELLOW)Set it:$(RESET) export AWS_REGION=us-east-1\n"; \
		exit 1; \
	fi

check-docker-env:
	@if [ -z "$(DOCKER_IMAGE)" ]; then \
		printf "$(RED)Error:$(RESET) DOCKER_IMAGE not set\n"; \
		printf "$(YELLOW)Set it:$(RESET) export DOCKER_IMAGE=docker.io/username/image\n"; \
		exit 1; \
	fi

check-ec2-ip:
	@if [ -z "$(EC2_IP)" ]; then \
		printf "$(RED)Error:$(RESET) EC2_IP not set\n"; \
		printf "$(YELLOW)Usage:$(RESET) make deploy-app ENV=dev EC2_IP=54.123.45.67\n"; \
		exit 1; \
	fi

# Validate environment
validate-env:
	@if [ "$(ENV)" != "dev" ] && [ "$(ENV)" != "prod" ]; then \
		printf "$(RED)Error:$(RESET) Invalid environment '$(ENV)'\n"; \
		printf "$(YELLOW)Use:$(RESET) ENV=dev or ENV=prod\n"; \
		exit 1; \
	fi

help:
	@printf "\n$(BOLD)$(CYAN)Bi8s Project Makefile$(RESET)\n\n"
	@printf "$(BOLD)$(YELLOW)Required Environment Variables:$(RESET)\n"
	@printf "   $(CYAN)export$(RESET) PROJECT_NAME=bi8s\n"
	@printf "   $(CYAN)export$(RESET) AWS_REGION=us-east-1\n"
	@printf "   $(CYAN)export$(RESET) DOCKER_IMAGE=docker.io/username/image\n\n"
	@printf "$(BOLD)$(BLUE)Backend Management:$(RESET)\n"
	@printf "   $(GREEN)make init-backend ENV=dev$(RESET)           Initialize Terraform backend\n"
	@printf "   $(GREEN)make init-backend ENV=prod$(RESET)\n\n"
	@printf "$(BOLD)$(BLUE)Infrastructure Management:$(RESET)\n"
	@printf "   $(GREEN)make create-infra ENV=dev$(RESET)           Create infrastructure (plan + apply)\n"
	@printf "   $(GREEN)make create-infra ENV=prod$(RESET)\n"
	@printf "   $(GREEN)make update-infra ENV=dev$(RESET)           Update infrastructure (plan + apply)\n"
	@printf "   $(GREEN)make update-infra ENV=prod$(RESET)\n"
	@printf "   $(GREEN)make destroy-infra ENV=dev$(RESET)          Destroy infrastructure\n"
	@printf "   $(GREEN)make destroy-infra ENV=prod$(RESET)\n\n"
	@printf "$(BOLD)$(BLUE)Docker Operations:$(RESET)\n"
	@printf "   $(GREEN)make build-image$(RESET)                    Build Docker image\n"
	@printf "   $(GREEN)make push-image$(RESET)                     Build and push Docker image\n\n"
	@printf "$(BOLD)$(BLUE)Application Deployment:$(RESET)\n"
	@printf "   $(GREEN)make deploy-app ENV=dev EC2_IP=x$(RESET)    Deploy application to EC2\n"
	@printf "   $(GREEN)make deploy-app ENV=prod EC2_IP=x$(RESET)\n\n"
	@printf "$(BOLD)$(BLUE)Development:$(RESET)\n"
	@printf "   $(GREEN)make test$(RESET)                           Run tests\n"
	@printf "   $(GREEN)make lint$(RESET)                           Run linter\n"
	@printf "   $(GREEN)make format$(RESET)                         Format code\n"
	@printf "   $(GREEN)make build$(RESET)                          Build application\n"
	@printf "   $(GREEN)make run$(RESET)                            Run application\n"
	@printf "   $(GREEN)make clean$(RESET)                          Clean build artifacts\n\n"
	@printf "$(BOLD)$(MAGENTA)Examples:$(RESET)\n"
	@printf "   $(CYAN)make init-backend ENV=dev$(RESET)\n"
	@printf "   $(CYAN)make create-infra ENV=dev$(RESET)\n"
	@printf "   $(CYAN)make push-image$(RESET)\n"
	@printf "   $(CYAN)make deploy-app ENV=dev EC2_IP=54.123.45.67$(RESET)\n\n"

# Backend Initialization
init-backend: check-env validate-env
	@printf "\n$(BOLD)$(BLUE)Initializing Terraform backend for $(YELLOW)$(ENV)$(BLUE)...$(RESET)\n\n"
	@./scripts/init-backend.sh $(PROJECT_NAME) $(ENV) $(AWS_REGION)
	@printf "\n$(BOLD)$(GREEN)Backend initialized for $(YELLOW)$(ENV)$(RESET)\n\n"

# Infrastructure Management
create-infra: check-env validate-env
	@printf "\n$(BOLD)$(BLUE)Creating infrastructure for $(YELLOW)$(ENV)$(BLUE)...$(RESET)\n\n"
	@printf "$(CYAN)Step 1:$(RESET) Planning changes...\n\n"
	@./scripts/deploy.sh $(ENV) plan
	@printf "\n$(CYAN)Step 2:$(RESET) Applying changes...\n\n"
	@./scripts/deploy.sh $(ENV) apply
	@printf "\n$(BOLD)$(GREEN)Infrastructure created for $(YELLOW)$(ENV)$(RESET)\n\n"

update-infra: check-env validate-env
	@printf "\n$(BOLD)$(BLUE)Updating infrastructure for $(YELLOW)$(ENV)$(BLUE)...$(RESET)\n\n"
	@printf "$(CYAN)Step 1:$(RESET) Planning changes...\n\n"
	@./scripts/deploy.sh $(ENV) plan
	@printf "\n$(CYAN)Step 2:$(RESET) Applying changes...\n\n"
	@./scripts/deploy.sh $(ENV) apply
	@printf "\n$(BOLD)$(GREEN)Infrastructure updated for $(YELLOW)$(ENV)$(RESET)\n\n"

destroy-infra: check-env validate-env
	@printf "\n$(BOLD)$(RED)Destroying infrastructure for $(YELLOW)$(ENV)$(RED)...$(RESET)\n\n"
	@./scripts/deploy.sh $(ENV) destroy
	@printf "\n$(BOLD)$(GREEN)Infrastructure destroyed for $(YELLOW)$(ENV)$(RESET)\n\n"

# Docker Operations
build-image: check-docker-env
	@printf "\n$(BOLD)$(BLUE)Building Docker image...$(RESET)\n\n"
	@docker build -t $(DOCKER_IMAGE):latest -f Dockerfile .
	@docker tag $(DOCKER_IMAGE):latest $(DOCKER_IMAGE):$(shell git rev-parse --short HEAD 2>/dev/null || echo "local")
	@printf "\n$(BOLD)$(GREEN)Image built:$(RESET) $(CYAN)$(DOCKER_IMAGE):latest$(RESET)\n\n"

push-image: check-docker-env build-image
	@printf "\n$(BOLD)$(BLUE)Pushing Docker image to registry...$(RESET)\n\n"
	@docker push $(DOCKER_IMAGE):latest
	@docker push $(DOCKER_IMAGE):$(shell git rev-parse --short HEAD 2>/dev/null || echo "local")
	@printf "\n$(BOLD)$(GREEN)Image pushed:$(RESET) $(CYAN)$(DOCKER_IMAGE):latest$(RESET)\n\n"

# Application Deployment
deploy-app: validate-env check-ec2-ip
	@printf "\n$(BOLD)$(BLUE)Deploying application to $(YELLOW)$(ENV)$(BLUE) EC2...$(RESET)\n"
	@printf "$(CYAN)EC2 IP:$(RESET) $(EC2_IP)\n\n"
	@printf "$(CYAN)Connecting to EC2 and deploying...$(RESET)\n\n"
	@ssh ec2-user@$(EC2_IP) "cd /opt/$${PROJECT_NAME:-bi8s}/compose && ../scripts/deploy.sh"
	@printf "\n$(BOLD)$(GREEN)Application deployed to $(YELLOW)$(ENV)$(RESET)\n\n"

# Development
test:
	@printf "\n$(BOLD)$(BLUE)Running tests...$(RESET)\n\n"
	@cd appl && go test -v ./...
	@printf "\n$(BOLD)$(GREEN)Tests complete$(RESET)\n\n"

lint:
	@printf "\n$(BOLD)$(BLUE)Running linter...$(RESET)\n\n"
	@if command -v golangci-lint > /dev/null; then \
		cd appl && golangci-lint run; \
		printf "\n$(BOLD)$(GREEN)Linting complete$(RESET)\n\n"; \
	else \
		printf "$(YELLOW)golangci-lint not installed$(RESET)\n\n"; \
	fi

format:
	@printf "\n$(BOLD)$(BLUE)Formatting code...$(RESET)\n\n"
	@cd appl && go fmt ./...
	@cd appl && gofmt -s -w .
	@printf "$(BOLD)$(GREEN)Code formatted$(RESET)\n\n"

build:
	@printf "\n$(BOLD)$(BLUE)Building application...$(RESET)\n\n"
	@cd appl && go build -o bin/api ./cmd/api
	@printf "$(BOLD)$(GREEN)Build complete:$(RESET) $(CYAN)appl/bin/api$(RESET)\n\n"

run:
	@printf "\n$(BOLD)$(BLUE)Running application...$(RESET)\n\n"
	@cd appl && go run ./cmd/api

# Utilities
clean:
	@printf "\n$(BOLD)$(BLUE)Cleaning build artifacts...$(RESET)\n\n"
	@rm -f appl/bin/api appl/bin/api-linux appl/bin/main
	@rm -f coverage.out coverage.html
	@rm -f *.tar.gz
	@find . -name "*.tfplan" -delete
	@printf "$(BOLD)$(GREEN)Clean complete$(RESET)\n\n"
