# Default shell used by make recipes.
SHELL := /bin/sh

# Centralized variables keep commands DRY and easy to override.
APP_NAME ?= bi8s-go
DOCKER_REGISTRY ?= docker.io
IMAGE_REPO ?= xanderbilla/go/bi8s
IMAGE_NAME ?= $(DOCKER_REGISTRY)/$(IMAGE_REPO)
IMAGE_TAG ?= latest
PLATFORM ?= linux/amd64
PLATFORMS ?= linux/amd64,linux/arm64
COMPOSE_FILE ?= docker-compose.yml
AIR_BIN ?= air
GO_MAIN ?= ./cmd/api

DOCKER_BUILD_BASE = docker buildx build
DOCKER_BUILD_TAGS = -t $(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: help default run-air run-docker docker-down image image-push buildx-check

# Default target: run API with Air for live reload in local development.
default: run

help: ## Show simplified commands.
	@echo "run      Run API with Air (default)"
	@echo "docker   Run API with Docker Compose"
	@echo "stop     Stop Docker Compose services"
	@echo "build    Build local Docker image"
	@echo "publish  Build and push multi-platform image"

run: ## Run API with Air (default).
	@command -v $(AIR_BIN) >/dev/null 2>&1 || { echo "air is not installed. Install with: go install github.com/air-verse/air@latest"; exit 1; }
	$(AIR_BIN)

docker: ## Run API with Docker Compose.
	docker compose -f $(COMPOSE_FILE) up --build

stop: ## Stop Docker Compose services.
	docker compose -f $(COMPOSE_FILE) down

buildx-check: ## Ensure Docker buildx is available before multi-platform builds.
	@docker buildx version >/dev/null 2>&1 || { echo "Docker buildx is required but not available"; exit 1; }

build: buildx-check ## Build local Docker image.
	$(DOCKER_BUILD_BASE) --platform $(PLATFORM) $(DOCKER_BUILD_TAGS) --load .

publish: buildx-check ## Build and push multi-platform image.
	$(DOCKER_BUILD_BASE) --platform $(PLATFORMS) $(DOCKER_BUILD_TAGS) --push .

# Backward-compatible aliases for previous target names.
run-air: run
run-docker: docker
docker-down: stop
image: build
image-push: publish
