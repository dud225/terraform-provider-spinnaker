BINARY := terraform-provider-spinnaker
TF_PLUGIN_DIR := $(HOME)/.terraform.d/plugins/registry.terraform.io/dud225/spinnaker/$(shell git describe --tags --abbrev=0)/$(shell go env GOOS)_$(shell go env GOARCH)

.PHONY: help install testacc

default: help
help:
	@grep -E '^(.+)\:\ .*##\ (.+)' ${MAKEFILE_LIST} | sed 's/:.*##/#/' | column -t -c 2 -s '#' | sort

build: ## Compile the Terraform provider
	go build -o $(BINARY)

install: build ## Install the Terraform provider in the ~/.terraform.d/plugins directory
	mkdir -p $(TF_PLUGIN_DIR)
	mv $(BINARY) $(TF_PLUGIN_DIR)

test: ## Run the unit tests
	go test ./... -v $(TESTARGS) -timeout 120m

testacc: ## Run the acceptance tests
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m
