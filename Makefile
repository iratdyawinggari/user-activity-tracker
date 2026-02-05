.PHONY: build run test docker-up docker-down swagger

build:
	@echo "Building application..."
	@go build -o bin/main ./cmd/webserver

run:
	@echo "Starting application..."
	@go run ./cmd/webserver

test:
	@echo "Running tests..."
	@go test ./... -v

docker-up:
	@echo "Starting Docker containers..."
	@docker-compose up -d

docker-down:
	@echo "Stopping Docker containers..."
	@docker-compose down

swagger:
	@echo "Generating Swagger documentation..."
	@swag init -g cmd/webserver/main.go -o docs

migrate:
	@echo "Running database migrations..."
	@mysql -h 127.0.0.1 -P 3306 -u root -p < migrations/01_init_schema.sql

clean:
	@echo "Cleaning up..."
	@rm -rf bin/
	@rm -f coverage.out

all: swagger build

help:
	@echo "Available commands:"
	@echo "  build      - Build the application"
	@echo "  run        - Run the application"
	@echo "  test       - Run tests"
	@echo "  docker-up  - Start Docker containers"
	@echo "  docker-down- Stop Docker containers"
	@echo "  swagger    - Generate Swagger docs"
	@echo "  migrate    - Run database migrations"
	@echo "  clean      - Clean build artifacts"
	@echo "  all        - Generate docs and build"