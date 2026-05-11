.PHONY: help setup init-backend create-infra update-infra destroy-infra build-image push-image deploy-app clean test test-unit test-integration coverage lint format fmt-check vet staticcheck govulncheck tidy quality build run run-summary runbuild local-setup reindex openapi-validate tofu-plan tofu-apply docker-up docker-down docker-logs docker-prune

# Variables
PROJECT_NAME ?= $(shell echo $$PROJECT_NAME)
AWS_REGION ?= $(shell echo $$AWS_REGION)
DOCKER_IMAGE ?= $(shell echo $$DOCKER_IMAGE)
ENV ?= dev

# OS detection (Darwin = macOS, Linux = Linux)
OS := $(shell uname -s)
ifeq ($(OS),Darwin)
  OPEN_CMD := open
else
  OPEN_CMD := xdg-open
endif

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
	@printf "$(BOLD)$(YELLOW)NOTE:$(RESET) Available environments: $(CYAN)dev$(RESET) and $(CYAN)prod$(RESET)\n\n"
	@printf "$(BOLD)$(BLUE)Backend Management:$(RESET)\n"
	@printf "   $(GREEN)make init-backend ENV=dev$(RESET)           Initialize Terraform backend\n\n"
	@printf "$(BOLD)$(BLUE)Infrastructure Management:$(RESET)\n"
	@printf "   $(GREEN)make create-infra ENV=dev$(RESET)           Create infrastructure (plan + apply)\n"
	@printf "   $(GREEN)make update-infra ENV=dev$(RESET)           Update infrastructure (plan + apply)\n"
	@printf "   $(GREEN)make destroy-infra ENV=dev$(RESET)          Destroy infrastructure\n\n"
	@printf "$(BOLD)$(BLUE)Docker Operations:$(RESET)\n"
	@printf "   $(GREEN)make build-image$(RESET)                    Build Docker image\n"
	@printf "   $(GREEN)make push-image$(RESET)                     Build and push Docker image\n\n"
	@printf "$(BOLD)$(BLUE)Local Development:$(RESET)\n"
	@printf "   $(GREEN)make local-setup$(RESET)                    Create DynamoDB tables + S3 bucket for local dev\n"
	@printf "   $(GREEN)make reindex$(RESET)                        Backfill DynamoDB data into local OpenSearch indexes\n\n"
	@printf "$(BOLD)$(BLUE)Development:$(RESET)\n"
	@printf "   $(GREEN)make test$(RESET)                           Run tests\n"
	@printf "   $(GREEN)make build$(RESET)                          clean → test(quality checks) → build\n"
	@printf "   $(GREEN)make run$(RESET)                            prune → quality → build → compose up (detached) → reindex\n\n"
	@printf "   $(GREEN)make tidy$(RESET)                           Run go mod tidy in app/ and test/\n\n"
	@printf "$(BOLD)$(BLUE)Targeted Quality:$(RESET)\n"
	@printf "   $(GREEN)make lint$(RESET)                           Run golangci-lint\n"
	@printf "   $(GREEN)make test-unit$(RESET)                      Run unit tests only (./internal/...)\n"
	@printf "   $(GREEN)make test-integration$(RESET)               Run integration tests (test/integration/...)\n"
	@printf "   $(GREEN)make coverage$(RESET)                       Generate HTML coverage report\n"
	@printf "   $(GREEN)make openapi-validate$(RESET)               Validate docs/openapi.yaml\n\n"
	@printf "$(BOLD)$(BLUE)Compose Shortcuts:$(RESET)\n"
	@printf "   $(GREEN)make docker-up$(RESET)                      Start local stack + auto-reindex search\n"
	@printf "   $(GREEN)make docker-down$(RESET)                    Stop local stack and remove volumes\n"
	@printf "   $(GREEN)make docker-logs$(RESET)                    Tail local stack logs\n\n"
	@printf "$(BOLD)$(BLUE)OpenTofu Shortcuts:$(RESET)\n"
	@printf "   $(GREEN)make tofu-plan ENV=dev$(RESET)              Plan infra changes\n"
	@printf "   $(GREEN)make tofu-apply ENV=dev$(RESET)             Apply infra changes\n\n"
	@printf "$(BOLD)$(BLUE)Bootstrap:$(RESET)\n"
	@printf "   $(GREEN)make setup$(RESET)                          Install dev tools (golangci-lint, staticcheck, govulncheck, air)\n\n"

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
	@if [ "$(ENV)" = "prod" ]; then \
		printf "$(BOLD)$(RED)PROD destroy requested. Type 'destroy-prod' to confirm:$(RESET) "; \
		read confirm; \
		[ "$$confirm" = "destroy-prod" ] || { printf "$(YELLOW)Aborted.$(RESET)\n"; exit 1; }; \
	fi
	@./scripts/deploy.sh $(ENV) destroy
	@printf "\n$(BOLD)$(GREEN)Infrastructure destroyed for $(YELLOW)$(ENV)$(RESET)\n\n"

# Docker Operations
build-image: check-docker-env quality
	@printf "\n$(BOLD)$(BLUE)Building Docker image...$(RESET)\n\n"
	@docker build -t $(DOCKER_IMAGE):latest -f app/Dockerfile app
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
	@printf "\n$(BOLD)$(BLUE)Running recommended Go quality checks...$(RESET)\n\n"
	@cd app && go fmt ./...
	@$(MAKE) fmt-check
	@$(MAKE) vet
	@cd app && go test -race -cover ./...
	@$(MAKE) lint
	@$(MAKE) staticcheck
	@$(MAKE) govulncheck
	@printf "\n$(BOLD)$(GREEN)All quality checks passed$(RESET)\n\n"
	@printf "$(GREEN)✓ tests$(RESET)\n"

fmt-check:
	@cd app && test -z "$$(gofmt -l .)"
	@printf "$(GREEN)✓ gofmt-check$(RESET)\n"

quality: test

vet:
	@cd app && go vet ./...
	@printf "$(GREEN)✓ go vet$(RESET)\n"
lint:
	@if command -v golangci-lint > /dev/null && golangci-lint version 2>/dev/null | grep -qF 'v2.'; then \
		cd app && golangci-lint run; \
		printf "$(GREEN)✓ lint$(RESET)\n"; \
	else \
		printf "$(YELLOW)→ Installing golangci-lint v2...$(RESET)\n"; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; \
		cd app && $$(go env GOPATH)/bin/golangci-lint run; \
		printf "$(GREEN)✓ lint$(RESET)\n"; \
	fi

staticcheck:
	@if command -v staticcheck > /dev/null; then \
		cd app && staticcheck ./...; \
		printf "$(GREEN)✓ staticcheck$(RESET)\n"; \
	else \
		printf "$(YELLOW)→ Installing staticcheck...$(RESET)\n"; \
		go install honnef.co/go/tools/cmd/staticcheck@latest; \
		cd app && staticcheck ./...; \
		printf "$(GREEN)✓ staticcheck$(RESET)\n"; \
	fi

govulncheck:
	@if command -v govulncheck > /dev/null; then \
		cd app && govulncheck ./...; \
		printf "$(GREEN)✓ govulncheck$(RESET)\n"; \
	else \
		printf "$(YELLOW)→ Installing govulncheck...$(RESET)\n"; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
		cd app && govulncheck ./...; \
		printf "$(GREEN)✓ govulncheck$(RESET)\n"; \
	fi
format:
	@cd app && go fmt ./...
	@cd app && gofmt -s -w .
	@printf "$(GREEN)✓ format$(RESET)\n"

build: clean test
	@cd app && go build -o bin/api ./cmd/api
	@printf "$(GREEN)✓ build$(RESET) $(CYAN)app/bin/api$(RESET)\n"

runbuild: build
	@cd app && air

# `make run` orchestrates a clean, fully verified local stack:
#   1. Prune the previous stack (volumes + project images).
#   2. Run static quality gates: tidy, fmt-check, vet, lint, staticcheck,
#      govulncheck, test-unit, openapi-validate, build.
#      (test-integration runs AFTER the stack is up since it needs live services.)
#   3. Bring compose up in detached mode (api is rebuilt).
#   4. Wait for /v1/livez, then reindex search.
#   5. Run integration tests against the live stack.
# Compose runs detached but every quality step is verbose so failures surface.
run: docker-prune tidy fmt-check vet lint staticcheck govulncheck test-unit openapi-validate build
	@printf "\n$(BOLD)$(BLUE)[run] Starting local stack (detached, https+ui profiles)...$(RESET)\n\n"
	@./scripts/compose.sh -f docker-compose.local.yml --profile https up -d --build
	@printf "\n$(BLUE)[run] Waiting for API readiness...$(RESET)\n"
	@for i in $$(seq 1 60); do \
		if curl -fsS http://localhost:8080/v1/livez > /dev/null 2>&1; then \
			printf "$(GREEN)✓ API is ready$(RESET)\n"; \
			break; \
		fi; \
		if [ $$i -eq 60 ]; then \
			printf "$(RED)Error: API did not become ready in time$(RESET)\n"; \
			exit 1; \
		fi; \
		sleep 2; \
	done
	@$(MAKE) reindex
	@$(MAKE) test-integration
	@printf "\n$(BOLD)$(GREEN)✓ run complete — stack healthy and reindexed$(RESET)\n"
	@$(MAKE) --no-print-directory run-summary

# Pretty-print the local endpoints + helpful info after `make run`.
# Reads BUILD_VERSION/BUILD_COMMIT from the wrapper so the banner reflects
# the actual image that was deployed.
run-summary:
	@VER=$$(cat VERSION 2>/dev/null || echo dev); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown); \
	if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then COMMIT="$$COMMIT-dirty"; fi; \
	printf "\n$(BOLD)$(CYAN)═══════════════════════════════════════════════════════════════$(RESET)\n"; \
	printf "$(BOLD)$(CYAN)  bi8s local stack — $(GREEN)v$$VER$(CYAN) ($(YELLOW)$$COMMIT$(CYAN))$(RESET)\n"; \
	printf "$(BOLD)$(CYAN)═══════════════════════════════════════════════════════════════$(RESET)\n\n"; \
	printf "$(BOLD)Application$(RESET)\n"; \
	printf "  API (HTTPS, via nginx)  $(GREEN)https://localhost/api/v1/$(RESET)\n"; \
	printf "  API (direct)            $(GREEN)http://localhost:8080/v1/$(RESET)\n"; \
	printf "  Swagger / OpenAPI       $(GREEN)https://localhost/api/v1/docs$(RESET)\n"; \
	printf "  Liveness                $(GREEN)https://localhost/api/v1/livez$(RESET)\n"; \
	printf "  Readiness               $(GREEN)https://localhost/api/v1/readyz$(RESET)\n\n"; \
	printf "$(BOLD)Observability$(RESET)\n"; \
	printf "  Grafana                 $(GREEN)http://localhost:$${GRAFANA_PORT:-4000}$(RESET)  (admin / admin)\n"; \
	printf "  Prometheus              $(GREEN)http://localhost:$${PROMETHEUS_PORT:-9090}$(RESET)\n"; \
	printf "  Tempo (traces)          $(GREEN)http://localhost:$${TEMPO_HTTP_PORT:-3200}$(RESET)\n"; \
	printf "  Loki (logs)             $(GREEN)http://localhost:$${LOKI_PORT:-3100}$(RESET)\n"; \
	printf "  cAdvisor                $(GREEN)http://localhost:$${CADVISOR_PORT:-8090}$(RESET)\n"; \
	printf "  node-exporter           $(GREEN)http://localhost:$${NODE_EXPORTER_PORT:-9100}/metrics$(RESET)\n\n"; \
	printf "$(BOLD)Data services$(RESET)\n"; \
	printf "  Redis                   $(CYAN)localhost:$${REDIS_PORT:-6379}$(RESET)\n"; \
	printf "  OpenSearch              $(GREEN)http://localhost:$${OPENSEARCH_PORT:-9200}$(RESET)\n\n"; \
	printf "$(BOLD)Useful commands$(RESET)\n"; \
	printf "  Tail API logs           $(YELLOW)make docker-logs$(RESET)\n"; \
	printf "  Stop & wipe stack       $(YELLOW)make docker-prune$(RESET)\n"; \
	printf "  Re-run search seed      $(YELLOW)make reindex$(RESET)\n\n"; \
	printf "$(YELLOW)Note: the local TLS cert is self-signed. Browsers will warn until$(RESET)\n"; \
	printf "$(YELLOW)you trust infra/docker/nginx/ssl/live/cert.crt locally.$(RESET)\n\n"

# Tear down the previous local stack and remove its built images so the next
# build is fully fresh. Safe to run repeatedly.
docker-prune:
	@printf "\n$(BOLD)$(BLUE)[run] Pruning previous local stack...$(RESET)\n\n"
	@./scripts/compose.sh -f docker-compose.local.yml --profile https down -v --remove-orphans --rmi local || true
	@printf "$(GREEN)✓ docker-prune$(RESET)\n"

tidy:
	@printf "\n$(BOLD)$(BLUE)Tidying Go modules...$(RESET)\n\n"
	@cd app && go mod tidy
	@cd test && go mod tidy
	@printf "$(GREEN)✓ tidy$(RESET)\n"

# Local development bootstrap
local-setup:
	@printf "\n$(BOLD)$(BLUE)Setting up local dev resources...$(RESET)\n\n"
	@./scripts/local-setup.sh
	@printf "\n$(BOLD)$(GREEN)Local resources ready$(RESET)\n\n"

reindex:
	@printf "\n$(BOLD)$(BLUE)Reindexing DynamoDB data into OpenSearch...$(RESET)\n\n"
	@cd app && SEARCH_ENDPOINT=http://localhost:9200 go run ./cmd/reindex
	@printf "\n$(BOLD)$(GREEN)Reindex complete$(RESET)\n\n"

# Bootstrap dev tools
setup:
	@printf "\n$(BOLD)$(BLUE)Installing Go dev tools ($(OS))...$(RESET)\n\n"
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install github.com/air-verse/air@latest
	@printf "$(GREEN)✓ setup$(RESET)\n\n"

# Targeted test runs
test-unit:
	@printf "\n$(BOLD)$(BLUE)Running unit tests...$(RESET)\n\n"
	@cd app && go test -race -count=1 -short ./internal/...
	@printf "$(GREEN)✓ test-unit$(RESET)\n"

test-integration:
	@printf "\n$(BOLD)$(BLUE)Running integration tests...$(RESET)\n\n"
	@cd test && go test -race -count=1 -tags=integration ./integration/...
	@printf "$(GREEN)✓ test-integration$(RESET)\n"

coverage:
	@printf "\n$(BOLD)$(BLUE)Generating coverage report...$(RESET)\n\n"
	@cd app && go test -race -count=1 -covermode=atomic -coverprofile=coverage.out ./...
	@cd app && go tool cover -html=coverage.out -o coverage.html
	@cd app && go tool cover -func=coverage.out | tail -1
	@printf "$(GREEN)✓ coverage$(RESET) $(CYAN)app/coverage.html$(RESET)\n"

# OpenAPI validation (uses docker if no local linter is installed)
openapi-validate:
	@printf "\n$(BOLD)$(BLUE)Validating docs/openapi.yaml...$(RESET)\n\n"
	@if command -v redocly > /dev/null; then \
		redocly lint docs/openapi.yaml; \
	elif command -v swagger-cli > /dev/null; then \
		swagger-cli validate docs/openapi.yaml; \
	else \
		docker run --rm -v $(PWD):/spec redocly/cli lint /spec/docs/openapi.yaml; \
	fi
	@printf "$(GREEN)✓ openapi-validate$(RESET)\n"

# Local docker compose shortcuts. All invocations go through scripts/compose.sh
# which resolves BUILD_VERSION/BUILD_COMMIT/BUILD_DATE dynamically from the
# working tree so the api image always carries real ldflags values.
docker-up:
	@printf "\n$(BOLD)$(BLUE)Starting local stack...$(RESET)\n\n"
	@./scripts/compose.sh -f docker-compose.local.yml up -d --build
	@printf "$(BLUE)Waiting for API readiness...$(RESET)\n"
	@for i in $$(seq 1 60); do \
		if curl -fsS http://localhost:8080/v1/livez > /dev/null 2>&1; then \
			printf "$(GREEN)✓ API is ready$(RESET)\n"; \
			break; \
		fi; \
		if [ $$i -eq 60 ]; then \
			printf "$(RED)Error: API did not become ready in time$(RESET)\n"; \
			exit 1; \
		fi; \
		sleep 2; \
	done
	@$(MAKE) reindex
	@printf "$(GREEN)✓ docker-up$(RESET)\n"

docker-down:
	@printf "\n$(BOLD)$(BLUE)Stopping local stack...$(RESET)\n\n"
	@./scripts/compose.sh -f docker-compose.local.yml down -v
	@printf "$(GREEN)✓ docker-down$(RESET)\n"

docker-logs:
	@./scripts/compose.sh -f docker-compose.local.yml logs -f --tail=200

# OpenTofu shortcuts (delegates to scripts/deploy.sh for parity with CI)
tofu-plan: check-env validate-env
	@./scripts/deploy.sh $(ENV) plan

tofu-apply: check-env validate-env
	@./scripts/deploy.sh $(ENV) apply

# Utilities
clean:
	@rm -f app/bin/api app/bin/api-linux app/bin/main
	@rm -f app/coverage.out app/coverage.html
	@rm -f coverage.out coverage.html
	@rm -f *.tar.gz
	@find . -name "*.tfplan" -delete
	@printf "$(GREEN)✓ clean$(RESET)\n"
