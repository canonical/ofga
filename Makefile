GOPATH := $(shell go env GOPATH)
GOFUMPT := $(GOPATH)/bin/gofumpt
GOIMPORTS = $(GOPATH)/bin/goimports
STATICCHECK := $(GOPATH)/bin/staticcheck
GOVULNCHECK := $(GOPATH)/bin/govulncheck
GOSEC := $(GOPATH)/bin/gosec

.PHONY: help
help:  ## Print help about available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

$(GOFUMPT):
	go install mvdan.cc/gofumpt@v0.4.0

$(GOIMPORTS):
	go install golang.org/x/tools/cmd/goimports@latest

$(STATICCHECK):
	go install honnef.co/go/tools/cmd/staticcheck@v0.3.3

$(GOVULNCHECK):
	go install golang.org/x/vuln/cmd/govulncheck@latest

$(GOSEC):
	go install github.com/securego/gosec/v2/cmd/gosec@v2.13.1

.PHONY: lint
lint: $(GOFUMPT) $(STATICCHECK) $(GOVULNCHECK) $(GOSEC)  ## Run linter
	gofumpt -e -d .
	go vet ./...
	staticcheck ./...
	govulncheck ./...
	gosec -quiet -tests ./...

.PHONY: fmt
fmt: $(GOIMPORTS) $(GOFUMPT)  ## Reformat code
	goimports -local github.com/canonical/ofga -l -w .
	gofumpt -l -w .

.PHONY: test
test:  ## Run tests (runs 'go test ./...')
	go test ./...

.PHONY: test-coverage
test-coverage:
	go test -coverprofile /tmp/cover.out ./... && \
	go tool cover -html=/tmp/cover.out -o /tmp/cover.html && \
	xdg-open /tmp/cover.html

# This build target may not necessarily be used much depending on our deployment strategy, but is kept here nonetheless as a reference for how to 
# build while inserting the git commit into the version info
.PHONY: build
build: ## Set version info based on current commit and then build
	go build -mod readonly -v -ldflags="-X github.com/canonical/ofga/internal/version.GitCommit=$$(git rev-parse --verify HEAD)" ./cmd/main/
