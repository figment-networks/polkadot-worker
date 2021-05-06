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

all: prepare build

.PHONY: prepare
prepare:
	mkdir -p build
	rm -rf ./build
	git clone https://github.com/itering/scale.go.git ./build/scale.go
	rm -rf ./api/scale/networks
	mkdir ./api/scale/networks
	cp -R ./build/scale.go/network/* ./api/scale/networks
	rm -rf ./build

.PHONY: build
build: LDFLAGS += -X $(MODULE)/cmd/polkadot-worker/config.Timestamp=$(shell date +%s)
build: LDFLAGS += -X $(MODULE)/cmd/polkadot-worker/config.Version=$(VERSION)
build: LDFLAGS += -X $(MODULE)/cmd/polkadot-worker/config.GitSHA=$(GIT_SHA)
build:
	go build -o worker -ldflags '$(LDFLAGS)'  ./cmd/polkadot-worker


.PHONY: build-live
build-live: LDFLAGS += -X $(MODULE)/cmd/polkadot-worker/config.Timestamp=$(shell date +%s)
build-live: LDFLAGS += -X $(MODULE)/cmd/polkadot-worker/config.Version=$(VERSION)
build-live: LDFLAGS += -X $(MODULE)/cmd/polkadot-worker/config.GitSHA=$(GIT_SHA)
build-live:
	go build -o worker-live -ldflags '$(LDFLAGS)'  ./cmd/polkadot-live



.PHONY: pack-release
pack-release:
	@mkdir -p ./release
	@make build
	@mv ./worker ./release/worker
	@make build-live
	@mv ./worker-live ./release/worker-live
	@zip -r polkadot-worker ./release
	@rm -rf ./release

.PHONY: test
test:
	go clean -testcache
	go test `go list ./... | grep -v github.com/figment-networks/polkadot-worker/indexer | grep -v github.com/figment-networks/polkadot-worker/proxy` -timeout 30s
