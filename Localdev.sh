
---

### `Makefile`

```makefile
.PHONY: all dev-up dev-down dev-wait cli-install cli-build \
        cp-build cp-test ml-build ml-test lint fmt \
        docker-build-all k8s-apply clean help

# ─── Variables ────────────────────────────────────────────────────────────────
REGISTRY        ?= ghcr.io/your-org/idp-platform
TAG             ?= $(shell git rev-parse --short HEAD)
NAMESPACE       ?= idp-system
KUBE_CONTEXT    ?= k3s-dev

# ─── Colors ───────────────────────────────────────────────────────────────────
GREEN  := \033[0;32m
YELLOW := \033[0;33m
CYAN   := \033[0;36m
RESET  := \033[0m

# ─── Default ──────────────────────────────────────────────────────────────────
all: lint test docker-build-all

# ─── Dev Stack ────────────────────────────────────────────────────────────────
dev-up:
	@echo "$(GREEN)Starting local dev stack...$(RESET)"
	docker compose up -d
	@echo "$(GREEN)✓ Stack started$(RESET)"

dev-down:
	@echo "$(YELLOW)Stopping local dev stack...$(RESET)"
	docker compose down -v

dev-wait:
	@echo "$(CYAN)Waiting for services...$(RESET)"
	@./scripts/wait-for-services.sh

dev-logs:
	docker compose logs -f

# ─── CLI (Rust) ───────────────────────────────────────────────────────────────
cli-build:
	@echo "$(CYAN)Building idpctl CLI...$(RESET)"
	cd cli && cargo build --release
	@echo "$(GREEN)✓ CLI built: cli/target/release/idpctl$(RESET)"

cli-install: cli-build
	@echo "$(CYAN)Installing idpctl...$(RESET)"
	cp cli/target/release/idpctl /usr/local/bin/idpctl
	@echo "$(GREEN)✓ idpctl installed$(RESET)"

cli-test:
	cd cli && cargo test

cli-lint:
	cd cli && cargo clippy -- -D warnings
	cd cli && cargo fmt --check

# ─── Control Plane (Go) ───────────────────────────────────────────────────────
cp-build:
	@echo "$(CYAN)Building control-plane...$(RESET)"
	cd control-plane && go build -o bin/server ./cmd/server
	@echo "$(GREEN)✓ Control plane built$(RESET)"

cp-test:
	cd control-plane && go test -v -race -coverprofile=coverage.out ./...

cp-lint:
	cd control-plane && golangci-lint run ./...

cp-generate:
	cd control-plane && go generate ./...

# ─── ML Engine (Python) ───────────────────────────────────────────────────────
ml-install:
	cd ml-cost-engine && pip install -e ".[dev]"

ml-test:
	cd ml-cost-engine && pytest tests/ -v --cov=src

ml-lint:
	cd ml-cost-engine && ruff check src/ tests/
	cd ml-cost-engine && mypy src/

# ─── Docker ───────────────────────────────────────────────────────────────────
docker-build-all: docker-build-cp docker-build-ml docker-build-cli

docker-build-cp:
	docker build -t $(REGISTRY)/control-plane:$(TAG) ./control-plane
	docker tag $(REGISTRY)/control-plane:$(TAG) $(REGISTRY)/control-plane:latest

docker-build-ml:
	docker build -t $(REGISTRY)/ml-cost-engine:$(TAG) ./ml-cost-engine
	docker tag $(REGISTRY)/ml-cost-engine:$(TAG) $(REGISTRY)/ml-cost-engine:latest

docker-build-cli:
	docker build -t $(REGISTRY)/idpctl:$(TAG) ./cli
	docker tag $(REGISTRY)/idpctl:$(TAG) $(REGISTRY)/idpctl:latest

docker-push:
	docker push $(REGISTRY)/control-plane:$(TAG)
	docker push $(REGISTRY)/ml-cost-engine:$(TAG)

# ─── K8s ──────────────────────────────────────────────────────────────────────
k8s-apply:
	kubectl apply -k k8s/overlays/dev --context $(KUBE_CONTEXT)

k8s-delete:
	kubectl delete -k k8s/overlays/dev --context $(KUBE_CONTEXT)

k8s-status:
	kubectl get all -n $(NAMESPACE) --context $(KUBE_CONTEXT)

# ─── Terraform ────────────────────────────────────────────────────────────────
tf-init:
	cd infra/terraform/envs/dev && terraform init

tf-plan:
	cd infra/terraform/envs/dev && terraform plan

tf-apply:
	cd infra/terraform/envs/dev && terraform apply -auto-approve

# ─── Lint / Test / All ────────────────────────────────────────────────────────
lint: cli-lint cp-lint ml-lint

test: cli-test cp-test ml-test

fmt:
	cd cli && cargo fmt
	cd control-plane && gofmt -w .
	cd ml-cost-engine && ruff format src/ tests/

# ─── Clean ────────────────────────────────────────────────────────────────────
clean:
	rm -rf cli/target
	rm -rf control-plane/bin
	find . -name "*.pyc" -delete
	find . -name "__pycache__" -delete

# ─── Help ─────────────────────────────────────────────────────────────────────
help:
	@echo "$(CYAN)IDP Platform — Available targets:$(RESET)"
	@echo ""
	@echo "  $(GREEN)Dev Stack:$(RESET)"
	@echo "    make dev-up          Start local Docker Compose stack"
	@echo "    make dev-down        Stop local stack"
	@echo "    make dev-wait        Wait for all services"
	@echo ""
	@echo "  $(GREEN)CLI (Rust):$(RESET)"
	@echo "    make cli-build       Build idpctl binary"
	@echo "    make cli-install     Install idpctl to /usr/local/bin"
	@echo ""
	@echo "  $(GREEN)Control Plane (Go):$(RESET)"
	@echo "    make cp-build        Build control-plane binary"
	@echo "    make cp-test         Run Go tests"
	@echo ""
	@echo "  $(GREEN)ML Engine (Python):$(RESET)"
	@echo "    make ml-test         Run Python tests"
	@echo ""
	@echo "  $(GREEN)Docker:$(RESET)"
	@echo "    make docker-build-all Build all Docker images"
	@echo ""
	@echo "  $(GREEN)Misc:$(RESET)"
	@echo "    make lint            Lint all components"
	@echo "    make test            Test all components"
	@echo "    make clean           Clean build artifacts"