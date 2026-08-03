BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
COMMIT := $(shell git log -1 --format='%H')
APPNAME := pulsar

# do not override user values
ifeq (,$(VERSION))
  VERSION := $(shell git describe --exact-match 2>/dev/null)
  # if VERSION is empty, then populate it with branch name and raw commit hash
  ifeq (,$(VERSION))
    VERSION := $(BRANCH)-$(COMMIT)
  endif
endif

# Update the ldflags with the app, client & server names
ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=$(APPNAME) \
	-X github.com/cosmos/cosmos-sdk/version.AppName=$(APPNAME)d \
	-X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT)

BUILD_FLAGS := -ldflags '$(ldflags)'
GO_BUILD_TAGS ?= purego
GO_TAGS_FLAG := $(if $(strip $(GO_BUILD_TAGS)),-tags=$(GO_BUILD_TAGS),)
GOFLAGS_WITH_TAGS := $(strip $(GOFLAGS) $(GO_TAGS_FLAG))
GOFLAGS_WITH_LINT_TAGS := $(strip $(GOFLAGS_WITH_TAGS) -buildvcs=false)
GOLANGCI_LINT_CACHE ?= /tmp/golangci-lint-cache

##############
###  Test  ###
##############

test-unit:
	@echo Running unit tests...
	@go test $(GO_TAGS_FLAG) -mod=readonly -v -timeout 30m ./...

test-race:
	@echo Running unit tests with race condition reporting...
	@go test $(GO_TAGS_FLAG) -mod=readonly -v -race -timeout 30m ./...

test-cover:
	@echo Running unit tests and creating coverage report...
	@go test $(GO_TAGS_FLAG) -mod=readonly -v -timeout 30m -coverprofile=$(COVER_FILE) -covermode=atomic ./...
	@go tool cover -html=$(COVER_FILE) -o $(COVER_HTML_FILE)
	@rm $(COVER_FILE)

test-scripts:
	@echo Running script unit tests...
	@PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts -p 'test_*.py'

test-docker-topologies:
	@echo Validating generated wrapper topologies...
	@./scripts/test_docker_topologies.sh

test-wrapper-e2e:
	@./scripts/test_archive_wrapper_deployment.sh shared
	@./scripts/test_archive_wrapper_deployment.sh per-validator
	@./scripts/test_archive_wrapper_deployment.sh external

bench:
	@echo Running unit tests with benchmarking...
	@go test $(GO_TAGS_FLAG) -mod=readonly -v -timeout 30m -bench=. ./...

test: govet test-unit test-scripts
security: govulncheck

.PHONY: test test-unit test-race test-cover test-scripts test-docker-topologies test-wrapper-e2e bench security

#################
###  Install  ###
#################

all: install

install:
	@echo "--> ensure dependencies have not been modified"
	@go mod verify
	@echo "--> installing $(APPNAME)d"
	@go install $(GO_TAGS_FLAG) $(BUILD_FLAGS) -mod=readonly ./cmd/$(APPNAME)d

.PHONY: all install

##################
###  Protobuf  ###
##################

# Use this target if you do not want to use Ignite for generating proto files

proto-deps:
	@echo "Installing proto deps"
	@echo "Proto deps present, run 'go tool' to see them"

proto-gen:
	@echo "Generating protobuf files..."
	@ignite generate proto-go --yes

.PHONY: proto-gen

#################
###  Linting  ###
#################

lint:
	@echo "--> Running linter"
	@GOLANGCI_LINT_CACHE="$(GOLANGCI_LINT_CACHE)" GOFLAGS="$(GOFLAGS_WITH_LINT_TAGS)" go tool github.com/golangci/golangci-lint/cmd/golangci-lint run ./... --timeout 15m

lint-fix:
	@echo "--> Running linter and fixing issues"
	@GOLANGCI_LINT_CACHE="$(GOLANGCI_LINT_CACHE)" GOFLAGS="$(GOFLAGS_WITH_LINT_TAGS)" go tool github.com/golangci/golangci-lint/cmd/golangci-lint run ./... --fix --timeout 15m

.PHONY: lint lint-fix

###################
### Development ###
###################

govet:
	@echo Running go vet...
	@go vet $(GO_TAGS_FLAG) ./...

govulncheck:
	@echo Running govulncheck...
	@GOFLAGS="$(GOFLAGS_WITH_TAGS)" go run golang.org/x/vuln/cmd/govulncheck@latest ./...

.PHONY: govet govulncheck
