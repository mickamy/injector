APP_NAME  = injector
BUILD_DIR = bin

# Examples that the e2e target regenerates. The two MUST examples are
# regenerated with --must to produce MustNew* constructors.
EXAMPLES_PLAIN = simple returns name-conflict embedded
EXAMPLES_MUST  = with-error embedded-error

.PHONY: all build install uninstall clean test test-unit test-e2e lint

all: build

build:
	@echo "Building $(APP_NAME)..."
	go build -o $(BUILD_DIR)/$(APP_NAME) .

install:
	@echo "Installing $(APP_NAME)..."
	@bin_dir=$$(go env GOBIN); \
	if [ -z "$$bin_dir" ]; then \
		bin_dir=$$(go env GOPATH)/bin; \
	fi; \
	mkdir -p "$$bin_dir"; \
	echo "Installing to $$bin_dir/$(APP_NAME)"; \
	go build -o "$$bin_dir/$(APP_NAME)" .

uninstall:
	@echo "Uninstalling $(APP_NAME)..."
	@bin_dir=$$(go env GOBIN); \
	if [ -z "$$bin_dir" ]; then \
		bin_dir=$$(go env GOPATH)/bin; \
	fi; \
	echo "Removing $$bin_dir/$(APP_NAME)"; \
	rm -f "$$bin_dir/$(APP_NAME)"

clean:
	@echo "Cleaning up..."
	rm -rf $(BUILD_DIR)

test: test-unit test-e2e

test-unit:
	go test ./internal/... -race

test-e2e:
	@for ex in $(EXAMPLES_PLAIN); do \
		echo ">>> regenerate example/$$ex"; \
		go run . ./example/$$ex/... || exit 1; \
	done
	@for ex in $(EXAMPLES_MUST); do \
		echo ">>> regenerate example/$$ex --must"; \
		go run . --must ./example/$$ex/... || exit 1; \
	done
	go vet ./example/...
	go build ./example/...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint is not installed"; \
		exit 1; \
	}
	golangci-lint run
