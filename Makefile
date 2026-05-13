PROTO_DIR     := proto
AUTH_SVC      := services/auth
WORKSPACE_SVC := services/workspace

AUTH_PB_OUT      := $(AUTH_SVC)/gen/pb
WORKSPACE_PB_OUT := $(WORKSPACE_SVC)/gen/pb

# Do not globally export per-service .env values here; Goose variables can
# collide across services and cause migrations to run against the wrong DB.

.PHONY: help proto build run-auth run-workspace run-voice run-gen seed-voices tidy scylla-up scylla-init-schema scylla-ui scylla-all scylla-shell dev-start docker-up docker-down docker-logs

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ─── Code Generation ──────────────────────────────────────────────────────────

proto: ## Generate gRPC Go and TS code from proto files
	-@mkdir $(AUTH_PB_OUT) 2>nul || exit 0
	-@mkdir $(WORKSPACE_PB_OUT) 2>nul || exit 0
	cd $(PROTO_DIR)/auth && npx @bufbuild/buf generate
	cd $(PROTO_DIR)/workspace && npx @bufbuild/buf generate
	@echo "✓ Proto generated"

# ─── Build & Run ──────────────────────────────────────────────────────────────

build: ## Build the service binaries
	cd $(AUTH_SVC) && go build -o ../../bin/auth ./cmd/
	cd $(WORKSPACE_SVC) && go build -o ../../bin/workspace ./cmd/

run-auth: ## Start Auth service (requires ScyllaDB running - see scylla-up)
	cd $(AUTH_SVC) && go run ./cmd/

run-workspace: ## Start Workspace service (requires ScyllaDB running - see scylla-up)
	cd $(WORKSPACE_SVC) && go run ./cmd/

tidy: ## Tidy Go modules
	cd $(AUTH_SVC) && go mod tidy
	cd $(WORKSPACE_SVC) && go mod tidy
	go work sync

# ─── ScyllaDB Management ──────────────────────────────────────────────────────

scylla-up: ## Start ScyllaDB cluster in Docker (required before running auth service)
	podman compose up -d scylladb
	@echo "✓ ScyllaDB starting..."
	@echo "  Waiting for cluster to be ready (health check: 30s startup + up to 75s for retries)"
	@echo "  Monitor with: podman compose logs -f scylladb"

scylla-init-schema: ## Manually initialize ScyllaDB schema (usually auto-initializes on auth service startup)
	@echo "ScyllaDB schema automatically initializes when auth service starts."
	@echo "To manually initialize if needed, run:"
	@echo "  docker exec scylladb-dev cqlsh -u cassandra -p cassandra < services/auth/db/schema.cql"

scylla-ui: ## Start DBeaver Web UI for database management (web dashboard at http://localhost:8978)
	podman compose up -d dbeaver
	@echo "✓ DBeaver Web UI starting..."
	@echo "  Access at: http://localhost:8978"
	@echo "  • Select 'New Database Connection' to add ScyllaDB"
	@echo "  • Host: scylladb, Port: 9042, Database: cassandra"
	@echo "  • Username: cassandra, Password: cassandra"
	@echo "  Monitor with: podman compose logs -f dbeaver"

scylla-all: scylla-up scylla-ui ## Start ScyllaDB + DBeaver Web UI

scylla-shell: ## Open interactive CQL shell to ScyllaDB
	podman exec -it scylladb-dev cqlsh -u cassandra -p cassandra

dev-start: scylla-up ## Start ScyllaDB and auth service for local development
	@echo "Waiting 5 seconds for ScyllaDB health check..."
	@timeout /t 5 /nobreak
	$(MAKE) run-auth

dev-start-workspace: scylla-up ## Start ScyllaDB and workspace service for local development
	@echo "Waiting 5 seconds for ScyllaDB health check..."
	@timeout /t 5 /nobreak
	$(MAKE) run-workspace

# ─── Docker ───────────────────────────────────────────────────────────────────

docker-up: ## Start all containers (detached)
	podman compose --env-file .env up -d

docker-down: ## Stop and remove containers
	podman compose down

docker-logs: ## Tail logs for all services
	podman compose logs -f
