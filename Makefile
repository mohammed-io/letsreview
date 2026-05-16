.DEFAULT_GOAL := help

build:
	go build -o letsreview ./cmd/letsreview

test:
	GOCACHE=/private/tmp/letsreview-gocache go test ./...

vet:
	GOCACHE=/private/tmp/letsreview-gocache go vet ./...

clean:
	rm -f letsreview

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build    Build letsreview binary"
	@echo "  test     Run all tests"
	@echo "  vet      Run go vet"
	@echo "  clean    Remove built binary"
	@echo "  help     Show this help"
