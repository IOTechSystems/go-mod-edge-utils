# Copyright (C) 2023 IOTech Ltd
#
# SPDX-License-Identifier: Apache-2.0

.PHONY: test unittest lint

ARCH=$(shell uname -m)
GO=CGO_ENABLED=0 GO111MODULE=on go
GOLANGCI_LINT_VERSION=v2.14.0

tidy:
	go mod tidy

unittest: tidy
	$(GO) test ./... -coverprofile=coverage.out ./...

lint:
	@which golangci-lint >/dev/null || echo "WARNING: go linter not installed. To install, run\n  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/$(GOLANGCI_LINT_VERSION)/install.sh | sh -s -- -b \$$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)"
	@if [ "z${ARCH}" = "zx86_64" ] && which golangci-lint >/dev/null ; then golangci-lint run --config .golangci.yml ; else echo "WARNING: Linting skipped (not on x86_64 or linter not installed)"; fi

test: unittest lint
	$(GO) vet ./...
	gofmt -l $$(find . -type f -name '*.go'| grep -v "/vendor/")
	[ "`gofmt -l $$(find . -type f -name '*.go'| grep -v "/vendor/")`" = "" ]

vendor:
	go mod vendor
