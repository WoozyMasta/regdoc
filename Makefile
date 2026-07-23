GO               ?= go
LINTER           ?= golangci-lint
ALIGNER          ?= betteralign
VULNCHECK        ?= govulncheck
CYCLONEDX        ?= cyclonedx-gomod
CLI_DOCS         ?= CLI.md
BINARY           ?= regdoc
OUTPUT_DIR       ?= build
CGO_ENABLED      ?= 0
GOFLAGS          ?= -buildvcs=auto -trimpath
LDFLAGS          ?= -s -w
GOWORK           ?= off
MODULE_PATH      ?= $(shell GOWORK=off $(GO) list -m -f '{{.Path}}')
RELEASE_MATRIX   ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64
NATIVE_GOOS      := $(shell go env GOOS)
NATIVE_GOARCH    := $(shell go env GOARCH)
NATIVE_EXTENSION := $(if $(filter $(NATIVE_GOOS),windows),.exe,)
VERSION          := $(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)
COMMIT           := $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
DATE             := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
URL              ?= https://$(MODULE_PATH)
VERSION_PKG      := $(MODULE_PATH)/internal/version
LDFLAGS_X        := -X '$(VERSION_PKG).Version=$(VERSION)' -X '$(VERSION_PKG).Commit=$(COMMIT)' -X '$(VERSION_PKG).BuildTime=$(DATE)' -X '$(MODULE_PATH).URL=$(URL)'

RACE ?= 0
ifeq ($(RACE),1)
	EXTRA_BUILD_FLAGS := -race
endif

export GOWORK

.PHONY: clean build release

clean:
	rm -rf $(OUTPUT_DIR)

build: clean $(BUILD_TOOLS)
	@mkdir -p $(OUTPUT_DIR)
	@echo ">> building native: regdoc$(NATIVE_EXTENSION)"
	GOOS=$(NATIVE_GOOS) GOARCH=$(NATIVE_GOARCH) \
	GOWORK=$(GOWORK) CGO_ENABLED=$(CGO_ENABLED) \
	$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS) $(LDFLAGS_X)" $(EXTRA_BUILD_FLAGS) \
	-o $(OUTPUT_DIR)/regdoc$(NATIVE_EXTENSION) ./cmd/regdoc

release: clean tool-cyclonedx
	@mkdir -p $(OUTPUT_DIR)
	@for target in $(RELEASE_MATRIX); do \
		goos=$${target%%/*}; \
		goarch=$${target##*/}; \
		ext=$$( [ $$goos = "windows" ] && echo ".exe" || echo "" ); \
		out="$(OUTPUT_DIR)/regdoc-$${goos}-$${goarch}$$ext"; \
		echo ">> building $$out"; \
		GOOS=$$goos GOARCH=$$goarch \
		GOWORK=$(GOWORK) CGO_ENABLED=$(CGO_ENABLED) \
		$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS) $(LDFLAGS_X)" \
		-o $$out ./cmd/regdoc; \
		$(MAKE) --no-print-directory sbom-target GOOS=$$goos GOARCH=$$goarch BINARY_PATH="$$out"; \
	done
	@$(MAKE) --no-print-directory sbom-app

.PHONY: check ci

check: verify tidy fmt vulncheck vet lint-fix align-fix test test-race
ci: download tools verify tidy-check fmt-check vulncheck vet lint align test

.PHONY: test test-race

test:
	$(GO) test ./...

test-race:
	CGO_ENABLED=1 $(GO) test -race ./...

.PHONY: download verify vet tidy tidy-check fmt fmt-check vulncheck lint lint-fix align align-fix

download:
	$(GO) mod download

verify:
	$(GO) mod verify

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

tidy-check:
	@$(GO) mod tidy
	@git diff --stat --exit-code -- go.mod go.sum || ( \
		echo "go mod tidy: repository is not tidy"; \
		exit 1; \
	)

fmt:
	gofmt -w .

fmt-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		echo "$$files"; \
		echo "gofmt: files need formatting"; \
		exit 1; \
	fi

vulncheck:
	$(VULNCHECK) ./...

lint:
	$(LINTER) run ./...

lint-fix:
	$(LINTER) run --fix ./...

align:
	$(ALIGNER) ./...

align-fix:
	-$(ALIGNER) -apply ./...
	$(ALIGNER) ./...

.PHONY: tools tool-golangci-lint tool-betteralign tool-cyclonedx

tools: tool-golangci-lint tool-betteralign tool-govulncheck

tool-golangci-lint:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

tool-betteralign:
	$(GO) install github.com/dkorunic/betteralign/cmd/betteralign@latest

tool-govulncheck:
	$(GO) install golang.org/x/vuln/cmd/govulncheck@latest

tool-cyclonedx:
	$(GO) install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest

.PHONY: sbom-app sbom-target

sbom-app:
	@echo ">> SBOM (packages) $(OUTPUT_DIR)/$(BINARY).sbom.json"
	GOOS=$(NATIVE_GOOS) GOARCH=$(NATIVE_GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
	$(CYCLONEDX) app -json -packages -files -licenses \
		-output "$(OUTPUT_DIR)/$(BINARY).sbom.json" \
		-main cmd/regdoc .

sbom-target:
	@echo ">> SBOM (target) $(BINARY_PATH).sbom.json"
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
	$(CYCLONEDX) app -json -packages -files -licenses \
		-output "$(BINARY_PATH).sbom.json" \
		-main cmd/regdoc .

.PHONY: cli-docs

cli-docs:
	GOWORK=$(GOWORK) $(GO) run $(GOFLAGS) -ldflags="$(LDFLAGS)" ./cmd/regdoc \
		docs md --program-name "$(BINARY)" --style posix --template=table \
		--toc "$(CLI_DOCS)" --trim-descriptions

.PHONY: release-notes

release-notes:
	@awk '\
	/^<!--/,/^-->/ { next } \
	/^## \[[0-9]+\.[0-9]+\.[0-9]+\]/ { if (found) exit; found=1; next } \
	found { \
		if (/^## \[/) { exit } \
		if (/^$$/) { flush(); print; next } \
		if (/^\* / || /^- /) { flush(); buf=$$0; next } \
		if (/^###/ || /^\[/) { flush(); print; next } \
		sub(/^[ \t]+/, ""); sub(/[ \t]+$$/, ""); \
		if (buf != "") { buf = buf " " $$0 } else { buf = $$0 } \
		next \
	} \
	function flush() { if (buf != "") { print buf; buf = "" } } \
	END { flush() } \
	' CHANGELOG.md
