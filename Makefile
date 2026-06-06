.PHONY: build test testacc fmt vet install clean generate

build:
	go build -o terraform-provider-dnswiz .

test:
	go test -count=1 -timeout=60s ./...

# Acceptance tests hit a real dnswiz API. Set DNSWIZ_ENDPOINT and
# DNSWIZ_API_KEY in the environment before running. They create and
# destroy real resources on the target account.
testacc:
	TF_ACC=1 go test -count=1 -timeout=10m -v ./internal/provider/...

fmt:
	gofmt -s -w .
	terraform fmt -recursive ./examples

vet:
	go vet ./...

# Install the provider into the local Terraform plugin cache so dev_overrides
# in ~/.terraformrc can pick it up.
install: build
	@echo "drop this in ~/.terraformrc to use the local build:"
	@echo ""
	@echo "provider_installation {"
	@echo "  dev_overrides {"
	@echo "    \"doon-io/dnswiz\" = \"$(shell pwd)\""
	@echo "  }"
	@echo "  direct {}"
	@echo "}"

clean:
	rm -f terraform-provider-dnswiz
	rm -rf dist/

# Regenerates the docs/ tree that the Terraform Registry renders.
# Pulls descriptions from the provider schema and example HCL from
# examples/. Run after any schema change.
generate:
	go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
	$(shell go env GOPATH)/bin/tfplugindocs generate --provider-name dnswiz
