tools/mockgen:
	GOBIN=$(abspath tools) go install github.com/golang/mock/mockgen@v1.6.0

clean-tools:
	rm -rf tools

clean: clean-tools

.PHONY: generate
generate: tools/mockgen
	go generate ./...

ENVTEST = $(abspath tools)/setup-envtest
.PHONY: envtest
$(ENVTEST):
	GOBIN=$(abspath tools) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: test-unit
test-unit: generate $(ENVTEST)
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go run github.com/onsi/ginkgo/v2/ginkgo -p --nodes 4 --cover -r -randomize-all --randomize-suites ./...
