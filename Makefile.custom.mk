tools/mockgen:
	GOBIN=$(abspath tools) go install github.com/golang/mock/mockgen@v1.6.0

clean-tools:
	rm -rf tools

clean: clean-tools

.PHONY: generate-go
generate-go: tools/mockgen
	go generate ./...

# Add dependency on generated files, but leave default `test` target commands alone
.PHONY: test
test: generate-go
