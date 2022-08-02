SHELL := /bin/bash
TAG ?= :latest


.PHONY: all
all: fmt vet build test

.PHONY: test-with-coverage
test-with-coverage:
	$(info Run unit tests with coverage)
	go test -v -vet=off -race ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

.PHONY: docker-build
docker-build:
	$(info Build docker container)
	docker build -t glintpay/glint-cloud-config-server:latest . --no-cache

.PHONY: fmt
fmt:
	$(info Run code formatter)
	go fmt ./...

.PHONY: vet
vet:
	$(info Run code vetting)
	go vet -composites=false ./...

.PHONY: lint
lint:
	$(info Run golangci-lint...)
	@golangci-lint run --enable gocyclo

.PHONY: test
test:
	$(info Run tests)
	go test -v -race -tags live ./...

.PHONY: build
build:
	go build ./...

.PHONY: install
install:
	go install ./cmd/...

.PHONY: package
package:
	CGO_ENABLED=0 go build ./cmd/gccs
