# ragdesk — developer convenience targets
.PHONY: help up down logs ps tidy fmt vet test ai-deps

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

up: ## Build and start the full stack (detached)
	docker compose up --build -d

down: ## Stop the stack and remove containers
	docker compose down

logs: ## Tail logs from all services
	docker compose logs -f

ps: ## Show service status
	docker compose ps

tidy: ## Tidy Go modules
	cd api && go mod tidy

fmt: ## Format Go code
	cd api && gofmt -w .

vet: ## Vet Go code
	cd api && go vet ./...

test: ## Run Go tests
	cd api && go test ./...

ai-deps: ## Install Python AI service dependencies
	cd ai && python3 -m pip install -r requirements.txt
