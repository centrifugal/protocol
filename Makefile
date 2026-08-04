.PHONY: all generate test test-cover bench fuzz lint tidy

all: generate

# Regenerate Go code from client.proto (see generate.sh for required tools).
generate:
	bash generate.sh

test:
	go test -race -count=1 ./...

test-cover:
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

bench:
	go test -run=^$$ -bench=. -benchmem

# Run every fuzz target for a short time. CI runs them longer, see .github/workflows/fuzz.yml
# (the target list there is explicit, so that each target gets its own job).
fuzz:
	@for target in $$(grep -hoE '^func Fuzz[A-Za-z0-9_]+' *_test.go | awk '{print $$2}'); do \
		echo "==> $$target"; \
		go test -run=^$$ -fuzz=^$$target$$ -fuzztime=30s || exit 1; \
	done

lint:
	golangci-lint run --timeout 3m0s

tidy:
	go mod tidy
