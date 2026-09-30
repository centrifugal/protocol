.PHONY: all generate test test-cover bench fuzz lint sec cross tidy json-compat json-compat-update json-bench json-bench-doc protobuf-compat protobuf-compat-update protobuf-bench protobuf-bench-doc

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
	@for target in $$(grep -hoE '^func Fuzz[A-Za-z0-9_]+' cfjson/*_test.go | awk '{print $$2}'); do \
		echo "==> cfjson $$target"; \
		go test -run=^$$ -fuzz=^$$target$$ -fuzztime=30s ./cfjson || exit 1; \
	done
	@for target in $$(grep -hoE '^func Fuzz[A-Za-z0-9_]+' internal/cfjsoncmp/*_test.go | awk '{print $$2}'); do \
		echo "==> cfjsoncmp $$target"; \
		(cd internal/cfjsoncmp && go test -vet=off -run=^$$ -fuzz=^$$target$$ -fuzztime=30s .) || exit 1; \
	done
	@for target in $$(grep -hoE '^func Fuzz[A-Za-z0-9_]+' cfprotobuf/internal/testtypes/*_test.go | awk '{print $$2}'); do \
		echo "==> cfprotobuf $$target"; \
		go test -run=^$$ -fuzz=^$$target$$ -fuzztime=30s ./cfprotobuf/internal/testtypes || exit 1; \
	done
	@for target in $$(grep -hoE '^func Fuzz[A-Za-z0-9_]+' internal/cfprotobufcmp/*_test.go | awk '{print $$2}'); do \
		echo "==> cfprotobufcmp $$target"; \
		(cd internal/cfprotobufcmp && go test -vet=off -run=^$$ -fuzz=^$$target$$ -fuzztime=30s .) || exit 1; \
	done

# internal/cfjsoncmp is a module of its own which still has the easyjson and
# segmentio based JSON code this package used before cfjson. Its tests prove
# that the generated code is compatible with them, its benchmarks compare the
# two. -vet=off because the old generated code copies structs with locks.
json-compat:
	cd internal/cfjsoncmp && go test -vet=off -count=1 .

# Regenerate what json-compat checks after changing the cfjson generator.
json-compat-update:
	go run ./cfjson/cmd/cfjson -raw Raw -out internal/cfjsoncmp/types_cfjson.go internal/cfjsoncmp/types.go
	go run ./cfjson/cmd/cfjson -raw Raw -fold-keys -append AppendJSONFold -decode DecodeJSONFold \
		-out internal/cfjsoncmp/types_cfjson_fold.go internal/cfjsoncmp/types.go
	cd internal/cfjsoncmp && go test -vet=off -count=1 -run 'TestGoldenFile' -update .

# Writes internal/cfjsoncmp/bench.txt. To publish the numbers move the file to
# internal/cfjsoncmp/results/<machine>.txt and run json-bench-doc.
json-bench:
	cd internal/cfjsoncmp && go test -vet=off -run=^$$ -bench=. -benchtime=200ms -count=10 . | tee bench.txt

# Regenerate cfjson/BENCHMARKS.md from internal/cfjsoncmp/results.
json-bench-doc:
	bash internal/cfjsoncmp/benchdoc.sh

# internal/cfprotobufcmp is the same for Protobuf: a module which still has
# the vtprotobuf code this package used before cfprotobuf.
protobuf-compat:
	cd internal/cfprotobufcmp && go test -vet=off -count=1 .

# Regenerate what protobuf-compat checks after changing the cfprotobuf
# generator. The CF suffix keeps the generated methods apart from the ones
# vtprotobuf generated for the same types.
protobuf-compat-update:
	go run ./cfprotobuf/cmd/cfprotobuf -suffix CF -out internal/cfprotobufcmp/types_cfprotobuf.go \
		internal/cfprotobufcmp/types.go internal/cfprotobufcmp/raw.go

# Writes internal/cfprotobufcmp/bench.txt. To publish the numbers move the file
# to internal/cfprotobufcmp/results/<machine>.txt and run protobuf-bench-doc.
protobuf-bench:
	cd internal/cfprotobufcmp && go test -vet=off -run=^$$ -bench=. -benchtime=200ms -count=10 . | tee bench.txt

# Regenerate cfprotobuf/BENCHMARKS.md from internal/cfprotobufcmp/results.
protobuf-bench-doc:
	bash internal/cfprotobufcmp/benchdoc.sh

lint:
	golangci-lint run --timeout 3m0s

# Known vulnerabilities in the code this module can reach. gosec, the other
# security analyser, runs as a part of lint, see .golangci.yml.
sec:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Run the tests where int is 32 bits and where the byte order is big-endian.
# The second needs QEMU user emulation (qemu-user with binfmt, or Docker
# Desktop), CI sets it up.
cross:
	GOARCH=386 CGO_ENABLED=0 go test -count=1 ./...
	GOARCH=s390x CGO_ENABLED=0 go test -count=1 ./...

tidy:
	go mod tidy
