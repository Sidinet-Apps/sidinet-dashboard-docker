APP=sidinet-dashboard
VERSION?=0.1.0-dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-s -w -X github.com/sidinet/sidinet-dashboard-docker/internal/version.Version=$(VERSION) -X github.com/sidinet/sidinet-dashboard-docker/internal/version.Commit=$(COMMIT) -X github.com/sidinet/sidinet-dashboard-docker/internal/version.BuildDate=$(BUILD_DATE)
.PHONY: build test run clean docker-build
build:
	CGO_ENABLED=1 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(APP) ./cmd/dashboard
test:
	go test ./...
run:
	SIDINET_DATA_DIR=./data SIDINET_DATABASE=./data/database.sqlite go run ./cmd/dashboard
clean:
	rm -rf bin data/database.sqlite*
docker-build:
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE) -t sidinet/sidinet-dashboard-docker:dev .
