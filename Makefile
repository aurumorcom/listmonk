# Try to get the commit hash from 1) git 2) the VERSION file 3) fallback.
ifeq ($(OS),Windows_NT)
	LAST_COMMIT ?= $(shell git rev-parse --short HEAD 2>NUL)
	VERSION ?= $(or $(LISTMONK_VERSION),$(shell git describe --tags --abbrev=0 2>NUL),"v0.0.0")
	BUILDDATE ?= $(shell powershell -NoProfile -Command "Get-Date -Format 'yyyy-MM-ddTHH:mm:ssZ'")
else
	LAST_COMMIT := $(or $(shell git rev-parse --short HEAD 2> /dev/null),$(shell head -n 1 VERSION 2>/dev/null | grep -oP -m 1 "^[a-z0-9]+$$"),"")
	VERSION := $(or $(LISTMONK_VERSION),$(shell git describe --tags --abbrev=0 2> /dev/null),$(shell grep -oP 'tag: \Kv\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?' VERSION 2>/dev/null),"v0.0.0")
	BUILDDATE := $(if $(SOURCE_DATE_EPOCH),$(shell date -u -d @$(SOURCE_DATE_EPOCH) +"%Y-%m-%dT%H:%M:%S%z" 2>/dev/null),$(shell date -u +"%Y-%m-%dT%H:%M:%S%z" 2>/dev/null))
endif

BUILDSTR := ${VERSION} (\#${LAST_COMMIT} $(BUILDDATE))

export CGO_ENABLED=0

YARN ?= yarn
GOPATH ?= $(HOME)/go
STUFFBIN ?= $(GOPATH)/bin/stuffbin
FRONTEND_YARN_STAMP = frontend/node_modules/.installed
FRONTEND_DIST = frontend/dist
FRONTEND_EMAIL_BUILDER_DIST_FINAL = frontend/public/static/email-builder

FRONTEND_EMAIL_BUILDER = frontend/email-builder
FRONTEND_EMAIL_BUILDER_YARN_STAMP = $(FRONTEND_EMAIL_BUILDER)/node_modules/.installed
FRONTEND_EMAIL_BUILDER_DIST = $(FRONTEND_EMAIL_BUILDER)/dist

ifeq ($(OS),Windows_NT)
	FRONTEND_SRC_FILES ?= $(wildcard frontend/src/*) $(wildcard frontend/src/*/*) $(wildcard frontend/src/*/*/*)
	EMAIL_BUILDER_SRC_FILES ?= $(wildcard frontend/email-builder/src/*) $(wildcard frontend/email-builder/src/*/*)
else
	FRONTEND_SRC_FILES := $(shell find frontend/fontello frontend/public frontend/src -type f 2>/dev/null)
	EMAIL_BUILDER_SRC_FILES := $(shell find $(FRONTEND_EMAIL_BUILDER)/src -type f 2>/dev/null)
endif

FRONTEND_DEPS = \
	$(FRONTEND_YARN_STAMP) \
	$(FRONTEND_EMAIL_BUILDER_DIST_FINAL) \
	frontend/index.html \
	frontend/package.json \
	frontend/vite.config.js \
	frontend/.eslintrc.js \
	$(FRONTEND_SRC_FILES)

FRONTEND_EMAIL_BUILDER_DEPS = \
	$(FRONTEND_EMAIL_BUILDER_YARN_STAMP) \
	$(FRONTEND_EMAIL_BUILDER)/package.json \
	$(FRONTEND_EMAIL_BUILDER)/tsconfig.json \
	$(FRONTEND_EMAIL_BUILDER)/vite.config.ts \
	$(EMAIL_BUILDER_SRC_FILES)

BIN := listmonk
STATIC := config.toml.sample \
	schema.sql queries:/queries permissions.json \
	static/public:/public \
	static/email-templates \
	frontend/dist:/admin \
	i18n:/i18n

ifeq ($(OS),Windows_NT)
	SQL ?= $(wildcard *.sql) $(wildcard queries/*.sql)
	SRC ?= $(wildcard *.go) $(wildcard cmd/*.go) $(wildcard internal/*/*.go) $(wildcard models/*.go)
else
	SQL := $(shell find . -type f -name "*.sql" 2>/dev/null) $(shell find queries -type f -name "*.sql" 2>/dev/null)
	SRC := $(shell find . -type f -name "*.go" 2>/dev/null)
endif

.PHONY: build
build: $(BIN)

$(STUFFBIN):
	go install github.com/knadh/stuffbin/...

$(FRONTEND_YARN_STAMP): frontend/package.json $(wildcard frontend/yarn.lock)
	cd frontend && $(YARN) install
	touch $@

$(FRONTEND_EMAIL_BUILDER_YARN_STAMP): $(FRONTEND_EMAIL_BUILDER)/package.json $(wildcard $(FRONTEND_EMAIL_BUILDER)/yarn.lock)
	cd $(FRONTEND_EMAIL_BUILDER) && $(YARN) install
	touch $@

# Build the backend to ./listmonk.
$(BIN): $(SRC) go.mod go.sum schema.sql $(SQL) permissions.json
	go build -o ${BIN} -ldflags="-s -w -X 'main.buildString=${BUILDSTR}' -X 'main.versionString=${VERSION}'" ./cmd

# Run the backend in dev mode. The frontend assets in dev mode are loaded from disk from frontend/dist.
.PHONY: run
run:
	go run -ldflags="-s -w -X 'main.buildString=${BUILDSTR}' -X 'main.versionString=${VERSION}' -X 'main.frontendDir=frontend/dist'" ./cmd

# Build the JS frontend into frontend/dist.
$(FRONTEND_DIST): $(FRONTEND_DEPS)
	cd frontend && $(YARN) install && export VUE_APP_VERSION="${VERSION}" && $(YARN) build
	touch -c $(FRONTEND_DIST)

# Build the JS email-builder dist.
$(FRONTEND_EMAIL_BUILDER_DIST): $(FRONTEND_EMAIL_BUILDER_DEPS)
	cd $(FRONTEND_EMAIL_BUILDER) && $(YARN) install && export VUE_APP_VERSION="${VERSION}" && $(YARN) build
	touch -c $(FRONTEND_EMAIL_BUILDER_DIST)

# Copy the build assets to frontend.
$(FRONTEND_EMAIL_BUILDER_DIST_FINAL): $(FRONTEND_EMAIL_BUILDER_DIST)
	mkdir -p $(FRONTEND_EMAIL_BUILDER_DIST_FINAL)
	cp -r $(FRONTEND_EMAIL_BUILDER_DIST)/* $(FRONTEND_EMAIL_BUILDER_DIST_FINAL)
	touch -c $(FRONTEND_EMAIL_BUILDER_DIST_FINAL)

.PHONY: build-frontend
build-frontend: $(FRONTEND_EMAIL_BUILDER_DIST_FINAL) $(FRONTEND_DIST)

.PHONY: build-email-builder
build-email-builder: $(FRONTEND_EMAIL_BUILDER_DIST_FINAL)

# Run Go tests.
.PHONY: test
test:
	go test ./...

# Bundle all static assets including the JS frontend into the ./listmonk binary
# using stuffbin (installed with make deps).
.PHONY: dist
dist: $(STUFFBIN) build build-frontend pack-bin

# pack-releases runns stuffbin packing on the given binary. This is used
# in the .goreleaser post-build hook.
.PHONY: pack-bin
pack-bin: build-frontend $(BIN) $(STUFFBIN)
	$(STUFFBIN) -a stuff -in ${BIN} -out ${BIN} ${STATIC}

# Use goreleaser to do a dry run producing local builds.
.PHONY: release-dry
release-dry:
	goreleaser release --parallelism 1 --clean --snapshot --skip=publish

# Use goreleaser to build production releases and publish them.
.PHONY: release
release:
	goreleaser release --parallelism 1 --clean

ifeq ($(OS),Windows_NT)
	DEV_FRONTEND_BG = start "Listmonk Frontend" cmd /c "cd frontend && (pnpm dev || yarn dev || npm run dev)"
else
	DEV_FRONTEND_BG = (cd frontend && (pnpm dev || $(YARN) dev || npm run dev)) &
endif

# Spin up background DB, MailHog, and WAHA containers & safely initialize DB.
.PHONY: dev-deps init-dev-db
dev-deps init-dev-db:
	cd dev && docker compose up -d db mailhog waha
	go run ./cmd --config dev/config.toml --install --idempotent --yes

# Re-initialize the local dev database (drops DB container/volume & re-creates clean schema).
.PHONY: re-init-dev-db reset-dev-db
re-init-dev-db reset-dev-db:
	cd dev && docker compose stop db && docker compose rm -f -v db && docker compose up -d db
	go run ./cmd --config dev/config.toml --install --idempotent --yes

# Run Vite frontend dev server locally with Hot Module Replacement (HMR).
.PHONY: dev-frontend run-frontend
dev-frontend run-frontend:
	cd frontend && (pnpm dev || $(YARN) dev || npm run dev)

# Run Go backend API server locally (starts background DB/deps if not already running).
.PHONY: dev-backend
dev-backend: dev-deps
	go run -ldflags="-s -w -X 'main.buildString=${BUILDSTR}' -X 'main.versionString=${VERSION}'" ./cmd --config=dev/config.toml

# Run full local development suite (spawns frontend and runs backend).
.PHONY: dev
dev:
	$(DEV_FRONTEND_BG)
	$(MAKE) dev-backend

# Build local docker images for development.
.PHONY: build-dev-docker
build-dev-docker: build ## Build docker containers for the entire suite (Front/Core/PG).
	cd dev; \
	docker compose build ; \

# Spin a local docker suite for local development.
.PHONY: dev-docker
dev-docker: build-dev-docker ## Build and spawns docker containers for the entire suite (Front/Core/PG).
	cd dev; \
	docker compose up

# Run the backend in docker-dev mode. The frontend assets in dev mode are loaded from disk from frontend/dist.
.PHONY: run-backend-docker
run-backend-docker:
	go run -ldflags="-s -w -X 'main.buildString=${BUILDSTR}' -X 'main.versionString=${VERSION}' -X 'main.frontendDir=frontend/dist'" ./cmd --config=dev/config.toml

# Tear down the complete local development docker suite.
.PHONY: rm-dev-docker
rm-dev-docker: build ## Delete the docker containers including DB volumes.
	cd dev; \
	docker compose down -v ; \

# Setup the db for local dev docker suite.
.PHONY: init-dev-docker
init-dev-docker: build-dev-docker ## Delete the docker containers including DB volumes.
	cd dev; \
	docker compose run --rm backend sh -c "make dist && ./listmonk --install --idempotent --yes --config dev/config.toml"
