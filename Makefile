include Makefile.tools
include .env

BINARY := faros-gittrack-controller
VERSION := $(shell git describe --always --dirty --tags 2>/dev/null || echo "undefined")

# Image URL to use all building/pushing image targets
IMG := faros-gittrack-controller:latest

.NOTPARALLEL:

.PHONY: all
all: test build

.PHONY: build
build: clean $(BINARY)

.PHONY: clean
clean:
	rm -f $(BINARY)

.PHONY: distclean
distclean: clean
	rm -rf vendor
	rm -rf release

# Generate code
.PHONY: generate
generate: vendor
	@ echo "\033[36mGenerating code\033[0m"
	$(GO) generate ./pkg/... ./cmd/...
	@ echo

# Verify generated code has been checked in
verify-%:
	@ make $*
	@ echo "\033[36mVerifying Git Status\033[0m"
	@ if [ "$$(git status -s)" != "" ]; then git --no-pager diff --color; echo "\033[31;1mERROR: Git Diff found. Please run \`make $*\` and commit the result.\033[0m"; exit 1; else echo "\033[32mVerified $*\033[0m";fi
	@ echo

# Run go fmt against code
.PHONY: fmt
fmt:
	$(GO) fmt ./pkg/... ./cmd/...

# Run go vet against code
.PHONY: vet
vet:
	$(GO) vet ./pkg/... ./cmd/...

.PHONY: lint
lint:
	@ echo "\033[36mLinting code\033[0m"
	$(LINTER) run --disable-all \
          --enable=vet \
          --enable=vetshadow \
          --enable=golint \
          --enable=ineffassign \
          --enable=goconst \
          --enable=deadcode \
          --enable=gofmt \
          --enable=goimports \
          --skip-dirs=pkg/client/ \
          --deadline=120s \
          --verbose \
          --tests ./...
		@ echo

# Run tests
export TEST_ASSET_KUBECTL := $(KUBEBUILDER)/kubectl
export TEST_ASSET_KUBE_APISERVER := $(KUBEBUILDER)/kube-apiserver
export TEST_ASSET_ETCD := $(KUBEBUILDER)/etcd

vendor:
	@ echo "\033[36mPuling dependencies\033[0m"
	$(DEP) ensure --vendor-only
	@ echo

.PHONY: test
test: vendor generate manifests
	@ echo "\033[36mRunning test suite in Ginkgo\033[0m"
	$(GINKGO) -v -race -randomizeAllSpecs ./pkg/... ./cmd/... -- -report-dir=$$ARTIFACTS
	@ echo

# Build manager binary
$(BINARY): generate fmt vet
	CGO_ENABLED=0 $(GO) build -o $(BINARY) -ldflags="-X main.VERSION=${VERSION}" github.com/pusher/faros/cmd/manager

# Build all arch binaries
release: test
	mkdir -p release
	GOOS=darwin GOARCH=amd64 go build -ldflags="-X main.VERSION=${VERSION}" -o release/$(BINARY)-darwin-amd64 github.com/pusher/faros/cmd/manager
	GOOS=linux GOARCH=amd64 go build -ldflags="-X main.VERSION=${VERSION}" -o release/$(BINARY)-linux-amd64 github.com/pusher/faros/cmd/manager
	GOOS=linux GOARCH=arm64 go build -ldflags="-X main.VERSION=${VERSION}" -o release/$(BINARY)-linux-arm64 github.com/pusher/faros/cmd/manager
	GOOS=linux GOARCH=arm GOARM=6 go build -ldflags="-X main.VERSION=${VERSION}" -o release/$(BINARY)-linux-armv6 github.com/pusher/faros/cmd/manager
	GOOS=windows GOARCH=amd64 go build -ldflags="-X main.VERSION=${VERSION}" -o release/$(BINARY)-windows-amd64 github.com/pusher/faros/cmd/manager
	$(SHASUM) -a 256 release/$(BINARY)-darwin-amd64 > release/$(BINARY)-darwin-amd64-sha256sum.txt
	$(SHASUM) -a 256 release/$(BINARY)-linux-amd64 > release/$(BINARY)-linux-amd64-sha256sum.txt
	$(SHASUM) -a 256 release/$(BINARY)-linux-arm64 > release/$(BINARY)-linux-arm64-sha256sum.txt
	$(SHASUM) -a 256 release/$(BINARY)-linux-armv6 > release/$(BINARY)-linux-armv6-sha256sum.txt
	$(SHASUM) -a 256 release/$(BINARY)-windows-amd64 > release/$(BINARY)-windows-amd64-sha256sum.txt
	$(TAR) -czvf release/$(BINARY)-$(VERSION).darwin-amd64.$(GOVERSION).tar.gz release/$(BINARY)-darwin-amd64
	$(TAR) -czvf release/$(BINARY)-$(VERSION).linux-amd64.$(GOVERSION).tar.gz release/$(BINARY)-linux-amd64
	$(TAR) -czvf release/$(BINARY)-$(VERSION).linux-arm64.$(GOVERSION).tar.gz release/$(BINARY)-linux-arm64
	$(TAR) -czvf release/$(BINARY)-$(VERSION).linux-armv6.$(GOVERSION).tar.gz release/$(BINARY)-linux-armv6
	$(TAR) -czvf release/$(BINARY)-$(VERSION).windows-amd64.$(GOVERSION).tar.gz release/$(BINARY)-windows-amd64

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	$(GO) run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	$(KUBECTL) apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	$(KUBECTL) apply -f config/crds
	$(KUSTOMIZE) build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: vendor
	@ echo "\033[36mGenerating manifests\033[0m"
	$(GO) run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
	@ echo

# Build the docker image
docker-build: test
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	$(SED) -i 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}
