LDFLAGS      := -w -s
MODULE       := github.com/figment-networks/polkadot-worker
VERSION_FILE ?= ./VERSION


# Git Status
GIT_SHA ?= $(shell git rev-parse --short HEAD)

ifneq (,$(wildcard $(VERSION_FILE)))
VERSION ?= $(shell head -n 1 $(VERSION_FILE))
else
VERSION ?= n/a
endif

all: build

.PHONY: build
build: LDFLAGS += -X $(MODULE)/cmd/worker-polkadot/config.Timestamp=$(shell date +%s)
build: LDFLAGS += -X $(MODULE)/cmd/worker-polkadot/config.Version=$(VERSION)
build: LDFLAGS += -X $(MODULE)/cmd/worker-polkadot/config.GitSHA=$(GIT_SHA)
build:
	go build -o worker -ldflags '$(LDFLAGS)'  ./cmd/worker-polkadot

.PHONY: pack-release
pack-release:
	@mkdir -p ./release
	@make build
	@mv ./worker ./release/worker
	@zip -r polkadot-worker ./release
	@rm -rf ./release

.PHONY: test
test: 
	go test ./...
