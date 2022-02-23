help: ## Show help messages.
	@grep -E '^[0-9a-zA-Z_-]+:(.*?## .*)?$$' $(MAKEFILE_LIST) | sed 's/^Makefile://' | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

NAME:=tmancer
build_tag:=$(shell git describe --tags 2> /dev/null)
BUILDFLAGS:="-s -w -X github.com/lzambarda/$(NAME)/internal.Version=$(build_tag)"

.PHONY: dependencies
dependencies: ## Install test and build dependencies
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.44.0

.PHONY: lint
lint: ## Hmmm, lint?
	go fmt ./...
	golangci-lint run ./...

.PHONY: build
build: generate ## Build the binary for both linux and macos, you can use "build build_tag=CUSTOM"
	@GOOS=darwin GOARCH=amd64 go build -ldflags $(BUILDFLAGS) -o ./bin/darwin/$(NAME) ./main.go
	@GOOS=linux GOARCH=amd64 go build -ldflags $(BUILDFLAGS) -o bin/linux/$(NAME) ./main.go

.PHONY: build_assets
build_assets: build ## Build and pack binaries as assets for github, you can use "build_assets build_tag=CUSTOM"
	@mkdir -p assets
	@tar -zcvf assets/darwin-amd64-$(NAME).tgz ./bin/darwin/$(NAME)
	@tar -zcvf assets/linux-amd64-$(NAME).tgz ./bin/linux/$(NAME)

.PHONY: generate
generate: ## Run stringer
	go generate ./...
