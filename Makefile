.PHONY: build run test clean docker docker-up docker-down help

# Binary name
BINARY=codementor
VERSION=0.1.0

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

# Default target
all: build

## build: Build the binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY) ./cmd/codementor

## run: Build and run the chat with current directory
run: build
	./$(BINARY) chat --path .

## serve: Build and run the HTTP server
serve: build
	./$(BINARY) serve

## test: Run tests
test:
	$(GOTEST) -v ./...

## clean: Clean build files
clean:
	$(GOCLEAN)
	rm -f $(BINARY)
	rm -rf .codementor/

## deps: Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

## lint: Run linter (requires golangci-lint)
lint:
	golangci-lint run

## docker: Build Docker image
docker:
	docker build -t codementor:$(VERSION) .
	docker tag codementor:$(VERSION) codementor:latest

## docker-up: Start with docker-compose
docker-up:
	docker-compose up -d

## docker-down: Stop docker-compose
docker-down:
	docker-compose down

## docker-logs: View docker logs
docker-logs:
	docker-compose logs -f

## index: Index current directory
index: build
	./$(BINARY) index --path .

## demo: Run a demo (requires Ollama)
demo: build
	@echo "🚀 Starting CodeMentor demo..."
	@echo "📂 Indexing current repository..."
	./$(BINARY) ask --path . "What are the main components of this project?"

## help: Show this help
help:
	@echo "CodeMentor - AI-Powered Code Repository Assistant"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

# Default
.DEFAULT_GOAL := help

