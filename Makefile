APP_NAME ?= clonacion-service
APP_PORT ?= 8080

.PHONY: run build test tidy lint
.PHONY: docker-up docker-down docker-logs docker-restart docker-clean
.PHONY: db-connect db-reset check-services

run:
	APP_NAME=$(APP_NAME) APP_PORT=$(APP_PORT) go run ./cmd/api

build:
	go build -o bin/$(APP_NAME) ./cmd/api

test:
	go test ./...

tidy:
	go mod tidy

lint:
	golangci-lint run ./...

# Docker Compose commands
docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

docker-restart:
	docker compose restart

docker-clean:
	docker compose down -v

# Database commands
db-connect:
	@if [ -f scripts/connect-db.sh ]; then \
		./scripts/connect-db.sh; \
	else \
		docker exec -it ms_facturacion_core_db psql -U postgres -d ms_facturacion_core; \
	fi

db-reset:
	docker compose down -v
	docker compose up -d --build

# Service verification
check-services:
	@if [ -f scripts/check-services.sh ]; then \
		./scripts/check-services.sh; \
	else \
		docker compose ps; \
		docker compose logs --tail=10; \
	fi

