// The oblikovati-mcp-bridge add-in: a c-shared library (.so/.dll) loaded by the
// host at runtime, exposing the Oblikovati API over MCP (HTTP/SSE) so an LLM can
// drive the live application. Its own module so the MCP SDK + net/http deps stay
// out of the cgo-free core. Built with its own Go toolchain — the runtime boundary
// to the host is the C ABI, not Go, so the versions are independent (see
// ../include/oblikovati_addin.h).
//
// The SHIPPED library links only the Apache-2.0 contract (oblikovati.org/api).
// The require on the GPL application module (oblikovati) is
// TEST-SCOPE ONLY — the bridge↔real-host integration tests (bridge/e2e_test.go,
// bridge/http_test.go) drive the live router/model. Both modules are sibling repos
// resolved by the go.work workspace at this repo's root (no committed replace).
module oblikovati.org/mcp-bridge

go 1.24.0

require (
	github.com/modelcontextprotocol/go-sdk v1.4.0
	oblikovati.org v0.0.0
	oblikovati.org/api v0.87.0
)

require (
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.5.3 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	golang.org/x/image v0.0.0-20211028202545-6944b10bf410 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
