# ==============================================================================
# Go Library Makefile — gourdianlogger
# ==============================================================================

# ----------------------------------------------------------------------
# Metadata
# ----------------------------------------------------------------------
MODULE          := github.com/gourdian25/gourdianlogger
VERSION         ?= v0.0.6
GO              := go
GOLANGCI_LINT   := golangci-lint

# ----------------------------------------------------------------------
# Directories
# ----------------------------------------------------------------------
COVERAGE_DIR    := test_coverage

# ----------------------------------------------------------------------
# Default target
# ----------------------------------------------------------------------
.DEFAULT_GOAL := help

# ----------------------------------------------------------------------
# Phony targets
# ----------------------------------------------------------------------
.PHONY: \
	help \
	test test-race bench \
	coverage coverage-unit coverage-integration coverage-all \
	coverage-summary \
	lint lint-fix \
	fmt vet tidy \
	clean clean-coverage \
	tag release

# ----------------------------------------------------------------------
# Help
# ----------------------------------------------------------------------
help:
	@echo ""
	@echo "Go Library Makefile — gourdianlogger"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Testing:"
	@echo "  test                    Run unit tests"
	@echo "  test-race               Run tests with race detector"
	@echo "  bench                   Run benchmarks"
	@echo ""
	@echo "Coverage:"
	@echo "  coverage                Generate full coverage HTML"
	@echo "  coverage-unit           Unit tests coverage only"
	@echo "  coverage-integration    Integration tests coverage only"
	@echo "  coverage-all            Generate all coverage reports"
	@echo "  coverage-summary        Print coverage summary"
	@echo ""
	@echo "Code quality:"
	@echo "  fmt                     Format code"
	@echo "  vet                     Run go vet"
	@echo "  lint                    Run golangci-lint"
	@echo "  lint-fix                Auto-fix lint issues"
	@echo ""
	@echo "Maintenance:"
	@echo "  tidy                    Go mod tidy"
	@echo "  clean                   Clean generated files"
	@echo ""
	@echo "Release:"
	@echo "  tag                     Create and push git tag"
	@echo "  release                 Tag + GoReleaser"
	@echo ""

# ----------------------------------------------------------------------
# Testing
# ----------------------------------------------------------------------
test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

bench:
	$(GO) test -bench=. -benchmem -run=^$$ ./...

# ----------------------------------------------------------------------
# Coverage
# ----------------------------------------------------------------------
$(COVERAGE_DIR):
	mkdir -p $(COVERAGE_DIR)

coverage: $(COVERAGE_DIR)
	$(GO) test -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out \
		-o $(COVERAGE_DIR)/coverage.html
	@echo "✔ Coverage report: $(COVERAGE_DIR)/coverage.html"

coverage-unit: $(COVERAGE_DIR)
	$(GO) test -coverprofile=$(COVERAGE_DIR)/unit.out \
		-run="^Test[^_]*$$" ./...
	$(GO) tool cover -html=$(COVERAGE_DIR)/unit.out \
		-o $(COVERAGE_DIR)/unit.html
	@echo "✔ Unit coverage report generated"

coverage-integration: $(COVERAGE_DIR)
	$(GO) test -coverprofile=$(COVERAGE_DIR)/integration.out \
		-run="Integration" ./...
	$(GO) tool cover -html=$(COVERAGE_DIR)/integration.out \
		-o $(COVERAGE_DIR)/integration.html
	@echo "✔ Integration coverage report generated"

coverage-all: coverage coverage-unit coverage-integration
	@echo "✔ All coverage reports generated in $(COVERAGE_DIR)/"

coverage-summary: $(COVERAGE_DIR)
	$(GO) test -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	$(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out

# ----------------------------------------------------------------------
# Code quality
# ----------------------------------------------------------------------
fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint:
	$(GOLANGCI_LINT) run

lint-fix:
	$(GOLANGCI_LINT) run --fix

tidy:
	$(GO) mod tidy

# ----------------------------------------------------------------------
# Cleanup
# ----------------------------------------------------------------------
clean-coverage:
	rm -rf $(COVERAGE_DIR)

clean: clean-coverage
	$(GO) clean -testcache

# ----------------------------------------------------------------------
# Release
# ----------------------------------------------------------------------
tag:
	git tag $(VERSION)
	git push origin $(VERSION)

release: tag
	goreleaser release --clean
