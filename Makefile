VERSION := v0.0.2

# Create test coverage directory
COVERAGE_DIR := test_coverage

.PHONY: build
build:
	go build -ldflags="-X github.com/gourdian25/gourdianlogger.Version=$(VERSION)" ./...

.PHONY: test
test:
	go test -v .

# Create coverage directory if it doesn't exist
$(COVERAGE_DIR):
	mkdir -p $(COVERAGE_DIR)

# Run all tests with coverage
.PHONY: coverage
coverage: $(COVERAGE_DIR)
	go test -coverprofile=$(COVERAGE_DIR)/coverage.out .
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "HTML coverage report saved as $(COVERAGE_DIR)/coverage.html"

# Run unit tests only (exclude integration, benchmark, and race tests)
.PHONY: coverage-unit
coverage-unit: $(COVERAGE_DIR)
	go test -coverprofile=$(COVERAGE_DIR)/unit_coverage.out -run="^Test[^_]*$$" .
	go tool cover -html=$(COVERAGE_DIR)/unit_coverage.out -o $(COVERAGE_DIR)/unit_coverage.html
	@echo "Unit test HTML coverage report saved as $(COVERAGE_DIR)/unit_coverage.html"

# Run integration tests only
.PHONY: coverage-integration
coverage-integration: $(COVERAGE_DIR)
	go test -coverprofile=$(COVERAGE_DIR)/integration_coverage.out -run="Integration" .
	go tool cover -html=$(COVERAGE_DIR)/integration_coverage.out -o $(COVERAGE_DIR)/integration_coverage.html
	@echo "Integration test HTML coverage report saved as $(COVERAGE_DIR)/integration_coverage.html"

# Run benchmark tests only
.PHONY: coverage-benchmark
coverage-benchmark: $(COVERAGE_DIR)
	go test -coverprofile=$(COVERAGE_DIR)/benchmark_coverage.out -run="Benchmark" .
	go tool cover -html=$(COVERAGE_DIR)/benchmark_coverage.out -o $(COVERAGE_DIR)/benchmark_coverage.html
	@echo "Benchmark test HTML coverage report saved as $(COVERAGE_DIR)/benchmark_coverage.html"

# Run race tests only
.PHONY: coverage-race
coverage-race: $(COVERAGE_DIR)
	go test -coverprofile=$(COVERAGE_DIR)/race_coverage.out -run="Race" -race .
	go tool cover -html=$(COVERAGE_DIR)/race_coverage.out -o $(COVERAGE_DIR)/race_coverage.html
	@echo "Race test HTML coverage report saved as $(COVERAGE_DIR)/race_coverage.html"

# Generate all coverage reports
.PHONY: coverage-all
coverage-all: coverage-unit coverage-integration coverage-benchmark coverage-race coverage
	@echo "All coverage reports generated in $(COVERAGE_DIR)/"

# Generate coverage summary for all test types
.PHONY: coverage-summary
coverage-summary: $(COVERAGE_DIR)
	@echo "=== Overall Coverage Summary ==="
	go test -coverprofile=$(COVERAGE_DIR)/coverage.out .
	go tool cover -func=$(COVERAGE_DIR)/coverage.out
	@echo ""
	@echo "=== Unit Tests Coverage Summary ==="
	go test -coverprofile=$(COVERAGE_DIR)/unit_coverage.out -run="^Test[^_]*$$" .
	go tool cover -func=$(COVERAGE_DIR)/unit_coverage.out
	@echo ""
	@echo "=== Integration Tests Coverage Summary ==="
	go test -coverprofile=$(COVERAGE_DIR)/integration_coverage.out -run="Integration" .
	go tool cover -func=$(COVERAGE_DIR)/integration_coverage.out

# Generate coverage summary for specific test type
.PHONY: coverage-summary-unit
coverage-summary-unit: $(COVERAGE_DIR)
	go test -coverprofile=$(COVERAGE_DIR)/unit_coverage.out -run="^Test[^_]*$$" .
	go tool cover -func=$(COVERAGE_DIR)/unit_coverage.out

.PHONY: coverage-summary-integration
coverage-summary-integration: $(COVERAGE_DIR)
	go test -coverprofile=$(COVERAGE_DIR)/integration_coverage.out -run="Integration" .
	go tool cover -func=$(COVERAGE_DIR)/integration_coverage.out

.PHONY: bench
bench:
	go test -bench=. -benchmem .

# Clean coverage files
.PHONY: clean-coverage
clean-coverage:
	rm -rf $(COVERAGE_DIR)

.PHONY: release
release:
	git tag $(VERSION)
	git push origin $(VERSION)
	goreleaser release --clean