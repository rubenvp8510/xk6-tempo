.PHONY: build test format clean run-example install-xk6 docker-build docker-push

# Docker image configuration
DOCKER_REGISTRY ?= quay.io
DOCKER_REPO ?= rvargasp
DOCKER_IMAGE ?= xk6-tempo
DOCKER_TAG ?= latest
DOCKER_FULL_IMAGE = $(DOCKER_REGISTRY)/$(DOCKER_REPO)/$(DOCKER_IMAGE):$(DOCKER_TAG)

# Find Go binary and add common paths to PATH
# Try to find Go in common locations
GOROOT := $(shell if [ -d /usr/local/go21 ]; then echo /usr/local/go21; elif [ -d /usr/local/go ]; then echo /usr/local/go; else echo ""; fi)
GOPATH := $(HOME)/go
GOBIN := $(GOPATH)/bin

# Add Go and Go bin to PATH
ifneq ($(GOROOT),)
export PATH := $(GOROOT)/bin:$(GOBIN):$(PATH)
else
export PATH := $(GOBIN):$(PATH)
endif

# Get Go command (use full path if GOROOT is set)
GO_CMD := $(if $(GOROOT),$(GOROOT)/bin/go,go)

# Check if xk6 is installed, install if not
install-xk6:
	@if ! command -v xk6 > /dev/null 2>&1; then \
		echo "Installing xk6..."; \
		$(GO_CMD) install go.k6.io/xk6/cmd/xk6@latest; \
	fi

# Build custom k6 binary with xk6-tempo extension
build: install-xk6 deps
	@echo "Building k6 with xk6-tempo extension..."
	xk6 build --with github.com/rvargasp/xk6-tempo=. --output ./k6

# Run go tests
test:
	$(GO_CMD) test ./...

# Format Go code
format:
	$(GO_CMD) fmt ./...

# Run example ingestion test
run-ingestion:
	./k6 run --env TEMPO_ENDPOINT=http://localhost:4318 examples/ingestion-test.js

# Run example query test
run-query:
	./k6 run --env TEMPO_ENDPOINT=http://localhost:3200 examples/query-test.js

# Run combined test
run-combined:
	./k6 run --env TEMPO_ENDPOINT=http://localhost:4318 examples/combined-test.js

# Clean build artifacts
clean:
	rm -f k6

# Install dependencies
deps:
	$(GO_CMD) mod download
	$(GO_CMD) mod tidy

# Build Docker image
docker-build:
	@echo "Building Docker image $(DOCKER_FULL_IMAGE)..."
	docker build -t $(DOCKER_FULL_IMAGE) .

# Push Docker image to registry
docker-push: docker-build
	@echo "Pushing Docker image $(DOCKER_FULL_IMAGE)..."
	docker push $(DOCKER_FULL_IMAGE)

