.PHONY: generate-go
generate-go:
	go generate ./...

# Add dependency on generated files, but leave default `test` target commands alone
.PHONY: test
test: generate-go
