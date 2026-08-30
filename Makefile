.PHONY: dev-setup dev-infra dev-infra-down dev-infra-clean dev-all dev-clean check-infra-ports check-docker check-service-port build run test test-integration lint dev-k8s dev-k8s-down help

dev-setup: dev-infra ## One-time bootstrap - bring up infra, ready to start building
	@echo "Infra is up (Redis, Postgres). Run 'nix develop' (or let direnv load it) to get the toolchain."

dev-infra: check-docker check-infra-ports ## Bring up infra only (Redis, Postgres), no services
	docker compose -f docker-compose.dev.yml up -d --wait

dev-infra-down: ## Stop infra, keep volumes
	docker compose -f docker-compose.dev.yml down

dev-infra-clean: ## Stop infra and wipe its volumes
	docker compose -f docker-compose.dev.yml down -v

dev-all: dev-infra ## Bring up infra plus every service and the frontend dev server
	@echo "Not implemented yet - run each service individually: make run SVC=<name>"

dev-clean: dev-infra-clean ## Full teardown - infra, volumes, everything dev-setup created

check-infra-ports: ## Warn if Redis/Postgres host ports are already taken before bringing infra up
	@./scripts/check-infra-ports.sh

check-docker: ## Verify Docker is installed and its daemon is reachable
	@./scripts/check-docker.sh

# --- Per-service dev conventions ---
# Port each service binds in local dev. Picked arbitrarily - no doc
# precedent - open to change. New services append one line here.
PORT_feed-simulator := 8080
PORT_ingestion-service := 8081

GO ?= go

check-service-port: ## Pre-flight port check for one service - make check-service-port SVC=<name>
	@test -n "$(SVC)" || { echo "SVC=<name> is required"; exit 1; }
	@test -n "$(PORT_$(SVC))" || { echo "no PORT_$(SVC) defined in Makefile - add one next to PORT_feed-simulator"; exit 1; }
	@./scripts/check-infra-ports.sh $(PORT_$(SVC))

build: ## Build a service binary - make build SVC=<name>
	@test -n "$(SVC)" || { echo "SVC=<name> is required"; exit 1; }
	cd services/$(SVC)/cmd/$(SVC) && $(GO) build -o $(CURDIR)/bin/$(SVC) .

run: check-service-port ## Run a service with Air hot-reload - make run SVC=<name>
	cd services/$(SVC)/cmd/$(SVC) && air -c $(CURDIR)/.air.toml

test: ## Run unit tests for a service - make test SVC=<name>
	@test -n "$(SVC)" || { echo "SVC=<name> is required"; exit 1; }
	$(GO) test ./services/$(SVC)/...

test-integration: check-docker ## Run integration tests for a service - make test-integration SVC=<name>
	@test -n "$(SVC)" || { echo "SVC=<name> is required"; exit 1; }
	$(GO) test -tags=integration ./services/$(SVC)/...

lint: ## Lint a service - make lint SVC=<name>
	@test -n "$(SVC)" || { echo "SVC=<name> is required"; exit 1; }
	golangci-lint run ./services/$(SVC)/...

dev-k8s: check-docker ## Start the local K8s dev loop (Kind cluster + Tilt: Redis, Postgres, observability)
	kind get clusters | grep -qx matchflow || kind create cluster --name matchflow --config k8s/local/kind-config.yaml
	tilt up

dev-k8s-down: ## Tear down the local K8s dev loop (Tilt resources + delete the Kind cluster)
	tilt down
	kind delete cluster --name matchflow

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
