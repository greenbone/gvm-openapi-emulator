# SPDX-License-Identifier: GPL-3.0-or-later
# SPDX-FileCopyrightText: 2026 Greenbone AG

SHELL := /bin/bash

# Tools
GOIMPORTS = go run golang.org/x/tools/cmd/goimports@latest
GOFUMPT   = go run mvdan.cc/gofumpt@latest

# Project
BIN_DIR  ?= bin
APP_NAME ?= emulator
MAIN_PKG ?= ./cmd/emulator

# Docker
IMAGE_NAME ?= openapi-sample-emulator:local

# Run config
HOST ?= 0.0.0.0
PORT ?= 8086
SPEC_PATH ?= ./examples/openvasd/swagger.json # change the folder path for other examples
SAMPLES_DIR ?= ./examples/openvasd/sample

FALLBACK_MODE ?= openapi_examples
VALIDATION_MODE ?= required
DEBUG_ROUTES ?= false

.PHONY: all
all: test build

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$(APP_NAME) $(MAIN_PKG)

.PHONY: run
run: build
	@SERVER_PORT=$(PORT) \
	SPEC_PATH=$(SPEC_PATH) \
	SAMPLES_DIR=$(SAMPLES_DIR) \
	FALLBACK_MODE=$(FALLBACK_MODE) \
	VALIDATION_MODE=$(VALIDATION_MODE) \
	DEBUG_ROUTES=$(DEBUG_ROUTES) \
	./$(BIN_DIR)/$(APP_NAME)

.PHONY: test
test:
	@for pkg in $$(go list ./...); do \
		echo "Testing $$pkg"; \
		go test -v $$pkg || exit 1; \
	done

MODULE := github.com/ozgen/openapi-sample-emulator
COVER_EXCLUDES := '(^$(MODULE)/cmd$$|^$(MODULE)$$|/(cmd|logger|examples|docs)(/|$$))'

.PHONY: cover
cover:
	@echo "Running tests with coverage..."
	@go test -tags=testcover -coverprofile=coverage.out -covermode=atomic $(shell go list ./... | grep -vE $(COVER_EXCLUDES))
	@echo "==> Average coverage:"
	@go tool cover -func=coverage.out | tee coverage.txt | grep 'total:' | awk '{print $$3}'
	@go tool cover -html=coverage.out -o coverage.html

.PHONY: format
format:
	@echo "Formatting..."
	@$(GOIMPORTS) -l -w .
	@GOFUMPT_SPLIT_LONG_LINES=on $(GOFUMPT) -l -w ./internal ./cmd ./config
	@go fmt ./...

.PHONY: lint
lint: format
	@echo "Linting..."
	@go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run

.PHONY: tidy
tidy:
	@go mod tidy

.PHONY: docker-build
docker-build:
	@docker build -t $(IMAGE_NAME) .

.PHONY: docker-run
docker-run:
	@docker run --rm -p 8086:8086 \
		-e SERVER_PORT=8086 \
		-e SPEC_PATH=/work/swagger.json \
		-e SAMPLES_DIR=/work/sample \
		-e FALLBACK_MODE=$(FALLBACK_MODE) \
		-e VALIDATION_MODE=$(VALIDATION_MODE) \
		-e DEBUG_ROUTES=$(DEBUG_ROUTES) \
		-v "./examples/openvasd:/work:ro" \
		$(IMAGE_NAME)

.PHONY: compose-up
compose-up:
	@docker compose up -d --build

.PHONY: compose-down
compose-down:
	@docker compose down

.PHONY: clean
clean:
	@rm -rf $(BIN_DIR) coverage.out coverage.html
