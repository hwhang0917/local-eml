BINARY      := local-eml
PKG         := ./cmd/local-eml
DIST_DIR    := dist
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
GOFLAGS     := -trimpath
PORT        ?= 7878

# Pure-Go build (modernc.org/sqlite needs no CGO) — enables clean cross-compile.
export CGO_ENABLED := 0

PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

.PHONY: help install build go-build web-build web-dev run test tidy fmt vet check clean cross

help:
	@echo "Targets:"
	@echo "  install     npm install (web/) + go module download/verify"
	@echo "  build       web-build + go-build  (single self-contained binary)"
	@echo "  go-build    Go binary only (assumes web/dist/ is up to date)"
	@echo "  web-build   Build the Vue SPA into web/dist/"
	@echo "  web-dev     Run Vite dev server (proxies /api to the Go server)"
	@echo "  run         Run the server on PORT=$(PORT) (default 7878)"
	@echo "  test        Run unit tests"
	@echo "  check       fmt + vet + test"
	@echo "  tidy        go mod tidy"
	@echo "  fmt         gofmt -s -w"
	@echo "  vet         go vet ./..."
	@echo "  cross       Cross-compile binaries for all platforms into $(DIST_DIR)/"
	@echo "  clean       Remove built binaries"

install:
	cd web && npm install
	go mod download
	go mod verify

build: web-build go-build

go-build:
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINARY) $(PKG)

web-build:
	cd web && npm run build

web-dev:
	cd web && npm run dev

run:
	go run $(PKG) serve --port $(PORT)

test:
	go test ./... -race -count=1

tidy:
	go mod tidy

fmt:
	gofmt -s -w .

vet:
	go vet ./...

check: fmt vet test

cross: web-build $(DIST_DIR)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		out=$(DIST_DIR)/$(BINARY)-$$os-$$arch$$ext; \
		echo "  -> $$out"; \
		GOOS=$$os GOARCH=$$arch go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $$out $(PKG) || exit 1; \
	done

$(DIST_DIR):
	mkdir -p $(DIST_DIR)

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf $(DIST_DIR)
