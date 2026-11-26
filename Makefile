# Build the oblikovati-mcp-bridge add-in as a c-shared library (.so/.dll/.dylib) and
# install it into the host's add-ins directory, which the host scans at startup.
#
# Its own Go toolchain: the add-in talks to the host over the C ABI, not Go, so its
# version is independent of the host. It tracks the head's (see go.mod) because the
# bridge<->host integration tests compile the app, which now requires 1.27.
export GOTOOLCHAIN := go1.27.0
export CGO_ENABLED := 1

OS := $(shell go env GOOS)
EXT := so
ifeq ($(OS),windows)
	EXT := dll
endif
ifeq ($(OS),darwin)
	EXT := dylib
endif

LIB := oblikovati-mcp-bridge.$(EXT)
OUT ?= build
# Where the host looks for add-ins (the head in the sibling app repo; OBK_ADDINS_DIR
# overrides at run). The app is a SIBLING of this repo — one `..` — matching the
# `replace oblikovati.org => ../Oblikovati` in go.work; two `..` escaped the workspace
# and silently installed outside it.
ADDINS_DIR ?= ../Oblikovati/head/addins
RUST_HOST ?= ../oblikovati-rust/target/debug/oblikovati$(if $(filter windows,$(OS)),.exe)

# The C ABI header is owned by the public oblikovati.org/api module (its source of truth);
# we vendor it into ./include (git-ignored) so cgo can -I it. Resolving the module dir
# with `go list -m` works both under go.work (local dev) and the CI -replace.
HDR := include/oblikovati_addin.h

.PHONY: build install test test-rust clean sync-header

sync-header: ## Vendor the C ABI header from the oblikovati.org/api module
	@mkdir -p include
	@api_dir=$$(go list -m -f '{{.Dir}}' oblikovati.org/api) && \
		cp "$$api_dir/include/oblikovati_addin.h" "$(HDR)" && \
		echo "synced $(HDR) <- $$api_dir/include"

build: sync-header ## Build the c-shared add-in into $(OUT)/
	mkdir -p $(OUT)
	go build -buildmode=c-shared -o $(OUT)/$(LIB) .

install: build ## Build then copy the library + manifest into $(ADDINS_DIR)
	mkdir -p $(ADDINS_DIR)
	cp $(OUT)/$(LIB) $(ADDINS_DIR)/
	cp manifest.json $(ADDINS_DIR)/
	@echo "installed $(LIB) -> $(ADDINS_DIR)"

test: sync-header ## Run the add-in tests (pure-Go bridge + full-stack E2E)
	go test ./...

test-rust: build ## Run MCP integration tests with the built Rust desktop
	OBK_RUST_MCP_HOST="$(abspath $(RUST_HOST))" OBK_RUST_MCP_LIBRARY="$(abspath $(OUT)/$(LIB))" \
		go test -count=1 -v ./integration/rusthost

clean: ## Remove build outputs and the vendored header
	rm -rf $(OUT) include
