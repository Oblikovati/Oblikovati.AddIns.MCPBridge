# MCP ↔ API parity

The MCP tool surface is **generated from the API**, which is the single source of
truth. Every `oblikovati.org/api/client` method carries `mcp:` doc directives; the
generator (`internal/mcpgen`) reads them plus each method's `c.call(wire.MethodX, …)`
call site and emits `bridge/tools_generated.go` — one registration per method. There
are **no hand-written tool registrations**.

## Directives (in the api/client method doc comments)

| directive | meaning |
|---|---|
| `mcp:tool <name>` | expose as an MCP tool named `<name>` |
| `mcp:summary <text>` | the LLM-facing description |
| `mcp:digest <fn>` | forward, then render the reply through bridge summarizer `<fn>` (token-efficient list/inspect tools) |
| `mcp:input <type>` | override the input DTO with a bridge-local type, for a cleaner MCP input schema (matrix-bearing or free-form inputs) |
| `mcp:image` | the tool returns an image (viewport/window capture) |
| `mcp:skip <reason>` | intentionally not a tool |

The request DTO and result type come from the call site / signature, so the emitted
registration is fully typed.

## Generation & parity gate

- Regenerate: `go generate ./...` (runs `internal/mcpgen`).
- `mcpgen` parses **every** `wire.Method*` constant and **fails** if any lacks an
  `mcp:tool`/`mcp:skip` annotation — so the surface can never drift below full parity.
- CI's `generate` job regenerates against `Oblikovati.API@develop` and fails if the
  committed `tools_generated.go` is stale.

## Hand-written infrastructure (not registrations)

These are referenced by the generated code but can't themselves be generated:

- the `addForward` / `addSummarized` / `addSummarizedIn` / `addCapture` helpers
  (`tools.go`, `viewport.go`), including `addToolSafe`, which falls back to a
  permissive free-form-object input when the MCP SDK can't derive a schema for a
  recursive wire DTO (e.g. a browser tree);
- the digest summarizer funcs (`summarize*` in `tools.go` / `tools_sketch.go`);
- the `mcp:input` override types (`addFeatureArg` in `tools.go`; the matrix-bearing
  assembly inputs in `tools_assembly.go`).

MCP **resources** (events, the logs resource) are a separate primitive, registered
from the server directly (`resources.go`), not from method annotations.
