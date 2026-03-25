.DEFAULT_GOAL := all

DIST_DIR=$(CURDIR)/dist
CONTAINER_IMAGE_BASE_REF="docker.elastic.co/observability-ci/apm-perf"
MODULE_DEPS=$(sort $(shell go list -deps -tags=darwin,linux,windows -f "{{with .Module}}{{if not .Main}}{{.Path}}{{end}}{{end}}" ./...))

all: test

fmt:
	@go tool github.com/elastic/go-licenser -license=Elasticv2 -exclude internal/telemetrygen .
	@go tool golang.org/x/tools/cmd/goimports -local github.com/elastic/ -w .

lint:
	go tool honnef.co/go/tools/cmd/staticcheck -checks=all ./...
	go list -m -json $(MODULE_DEPS) | go tool go.elastic.co/go-licence-detector \
		-includeIndirect \
		-rules tools/notice/rules.json
	find . -name go.mod -execdir sh -c 'go mod tidy -diff' sh {} +

.PHONY: clean
clean:
	rm -fr bin

.PHONY: build
build: COMMIT_SHA=$$(git rev-parse HEAD)
build: CURRENT_TIME_ISO=$$(date -u +"%Y-%m-%dT%H:%M:%SZ")
build: LDFLAGS=-X 'github.com/elastic/apm-perf/internal/version.commitSha=$(COMMIT_SHA)'
build: LDFLAGS+=-X 'github.com/elastic/apm-perf/internal/version.buildTime=$(CURRENT_TIME_ISO)'
build:
	mkdir -p $(DIST_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/apmsoak cmd/apmsoak/*.go
	go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/apmbench cmd/apmbench/*.go
	go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/apmtelemetrygen cmd/apmtelemetrygen/*.go

.PHONY: test
test: go.mod
	find . -name go.mod -execdir sh -c 'go test -race -v ./...' sh {} +

.PHONY: publish
publish: BASE_IMAGE_VERSION=$$(go list -m -f "{{.Version}}" go)
publish: COMMIT_SHA_SHORT=$$(git rev-parse --short HEAD)
publish: COMMIT_SHA=$$(git rev-parse HEAD)
publish: CURRENT_TIME_ISO=$$(date -u +"%Y-%m-%dT%H:%M:%SZ")
publish: CURRENT_TIME=$$(date +%s)
publish: IMAGE_ID=$${IMAGE_VERSION:-latest}-$(CURRENT_TIME)-$(COMMIT_SHA_SHORT)
publish: IMAGE_REF=$(CONTAINER_IMAGE_BASE_REF):$(IMAGE_ID)
publish: PROJECT_URL=$$(go list -m all | head -1)
publish:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--push \
		--build-arg base_image_version=$(BASE_IMAGE_VERSION) \
		--build-arg commit_sha=$(COMMIT_SHA) \
		--build-arg current_time=$(CURRENT_TIME_ISO) \
		--build-arg image_id=$(IMAGE_ID) \
		--build-arg project_url=$(PROJECT_URL) \
		-t $(IMAGE_REF) \
		-t $(CONTAINER_IMAGE_BASE_REF):latest \
		-f Containerfile \
		.

notice: NOTICE.txt
NOTICE.txt: go.mod
	go list -m -json $(MODULE_DEPS) | go tool go.elastic.co/go-licence-detector \
		-includeIndirect \
		-rules tools/notice/rules.json \
		-noticeTemplate tools/notice/NOTICE.txt.tmpl \
		-noticeOut NOTICE.txt
