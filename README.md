# oblikovati-mcp-bridge

The first Oblikovati add-in: an **MCP server** that exposes the Oblikovati API so
an LLM (or any MCP client) can drive the **live, running** application in real time
— creating documents, authoring sketches, building features, and reading the model
back. It is built as a **C-shared library** (`.so`/`.dll`/`.dylib`) and loaded
in-process by the host at startup.

Design rationale and the divergence from the gRPC plan:
[ADR-0016](../../Oblikovati/architecture/decisions/ADR-0016-shared-library-addins-mcp-bridge.md)
(amends [ADR-0003](../../Oblikovati/architecture/decisions/ADR-0003-extensibility-hybrid-rpc.md)).

## Build & run

```sh
# In this directory: build the library and copy it (+ manifest) into the host's
# add-ins folder (../../Oblikovati/head/addins, in the sibling app repo). Override
# ADDINS_DIR to install elsewhere.
make install

# Launch the host; it loads every add-in in ./addins at startup.
cd ../../Oblikovati/head && make run
```

The MCP endpoint defaults to `http://127.0.0.1:7800/mcp`. Override with
`OBK_MCP_ADDR` (host:port); override the scanned add-ins dir with `OBK_ADDINS_DIR`.

Smoke-test a running bridge (lists tools, reads the model tree):

```sh
go run ./cmd/mcpcheck            # or: go run ./cmd/mcpcheck --url http://host:port/mcp
```

## MCP tools

Each tool forwards a typed input to a host method and returns the method's JSON
result as text content. A host error becomes a tool error.

| Tool | Input | Host method |
|------|-------|-------------|
| `list_commands` | — | `commands.list` |
| `execute_command` | `{id}` | `commands.execute` |
| `list_documents` | — | `documents.list` |
| `create_document` | `{type, name}` (`type`: part\|assembly\|drawing\|presentation) | `documents.create` |
| `activate_document` | `{id}` | `documents.activate` |
| `list_parameters` | — | `parameters.list` |
| `get_parameter` | `{name}` | `parameters.get` |
| `add_parameter` | `{name, expression}` | `parameters.add` |
| `set_parameter` | `{name, expression}` | `parameters.set` |
| `get_model_tree` | — | `model.tree` |
| `get_selection` | — | `model.selection` |
| `create_sketch` | `{plane}` (`XY`\|`XZ`\|`YZ`) | `sketch.create` |
| `sketch_rectangle` | `{sketchIndex, width, height}` | `sketch.rectangle` |
| `list_feature_kinds` | — | `features.list` |
| `add_feature` | `{kind, args}` | `features.add` |
| `list_work_planes` | — | `workPlanes.list` |
| `create_work_plane` | `{type, …}` | `workPlanes.create` |
| `redefine_work_plane` | `{index, scalars?, repick?}` | `workPlanes.redefine` |
| `create_work_point` | `{at}` | `workPoints.create` |
| `list_appearances` | — | `appearances.list` |
| `get_appearance` | `{id}` | `appearances.get` |
| `create_appearance` | `{id, name}` | `appearances.create` |
| `update_appearance` | `{id, …}` | `appearances.update` |
| `list_materials` | — | `materials.list` |
| `get_material` | `{id}` | `materials.get` |
| `create_material` | `{id, name}` | `materials.create` |
| `update_material` | `{id, …}` | `materials.update` |
| `assign_material` | `{id, …}` | `model.assignMaterial` |
| `assign_appearance` | `{id, …}` | `model.assignAppearance` |
| `get_physical_properties` | — | `model.physicalProperties` |
| `create_command` | `{id, displayName, ribbon?, tab?, category?, environment?}` | `commands.create` |
| `get_ribbon` | — | `ribbon.list` |
| `get_active_theme` | — | `theme.active` |
| `list_themes` | — | `theme.list` |

The table above lists the core tools. The bridge also forwards the **full** host
surface (≈121 tools total); each tool maps 1:1 to the like-named `domain.method` and
the authoritative, always-current list is the MCP `tools/list` call. The additional
families are:

- **2D sketch authoring** (`sketch.*`): `list_sketches`, `get_sketch`,
  `list_sketch_entities` / `_constraints` / `_dimensions` / `_profiles`,
  `get_sketch_constraint_status`, `edit_sketch` / `exit_sketch` / `solve_sketch` /
  `delete_sketch`, `set_sketch_property`, `add_sketch_entity`, `offset_sketch`,
  `transform_sketch`, `add_sketch_pattern`, `add_sketch_text`, `add_fill_region`,
  `add_sketch_image`, `project_geometry`, `add_sketch_constraint` /
  `delete_sketch_constraint`, `add_sketch_dimension` / `drive_sketch_dimension`,
  `auto_dimension_sketch`. Enumerations return a one-line digest + full JSON.
- **3D sketch authoring** (`sketch3d.*`): the spatial analogue —
  `create_sketch3d`, `list_sketches3d`, `add_sketch3d_entity` / `_constraint` /
  `_dimension`, `list_sketch3d_paths` / `_profiles`, `include_sketch3d_geometry`,
  `include_2d_sketch_in_3d`, `add_sketch3d_surface_curve`, `transform_sketch3d`, …
- **View / lighting / environment** (`view.*`, `lighting.*`, `environment.*`):
  `get`/`set_display_mode`, `get`/`set_shadows`, `*_lighting_style`, `*_light`,
  `get`/`set_environment`, `load_environment_image`.
- **Viewport overlays** (`clientGraphics.*`, `interactionGraphics.*`):
  `set`/`list`/`delete`/`set_client_graphics_visible`, `update`/`clear_interaction_graphics`.
- **Session** (`transaction.*`, `model.referenceKeys`): `undo`, `redo`,
  `get_undo_state`, `get_reference_keys`.
- **Diagnostics** (`logs.tail`): `tail_logs` — a real-time, cursor-paged tail of the host
  operation trace (every command with timing + outcome, and panic+stack for caught kernel
  bugs). Tail with `sinceSeq=<previous nextSeq>` to watch the kernel work while driving or
  stress-testing it. Recovered panics are reported as errors, never crash the host.

> Note (MCP best practice): ≈121 tools is a large surface. Tool descriptions,
> token-efficient digests for noisy reads, and the discovery resources below keep it
> workable, but if agent tool-selection degrades, the next step is progressive
> disclosure (gate advanced families behind a capability flag, or expose the API via
> Anthropic's code-execution-with-MCP pattern) rather than growing the flat list.

Sizes/values are **unit-bearing expressions** (e.g. `"40 mm"`, `"5 cm"`,
`"width * 2"`). The parameter/sketch/feature tools act on the **active** document,
which must be a part.

**Build a solid from scratch:** `create_document` → `create_sketch {plane:"XY"}` →
`sketch_rectangle {sketchIndex:0, width:"40 mm", height:"30 mm"}` →
`add_feature {kind:"extrude", args:{sketchIndex:0, profileIndex:0, distance:"50 mm",
operation:"new"}}`.

## MCP resources (self-teaching)

| URI | Content |
|-----|---------|
| `oblikovati://docs/getting-started` | How to drive the bridge (markdown) |
| `oblikovati://docs/model` | The document/parameter/sketch/feature/body object model |
| `oblikovati://schema/feature-kinds` | **Live** feature operations + JSON arg schemas (`features.list`) |
| `oblikovati://schema/commands` | **Live** command list (`commands.list`) |
| `oblikovati://schema/sketches` / `…/sketches3d` | **Live** 2D / 3D sketch lists (`sketch.list` / `sketch3d.list`) |
| `oblikovati://schema/display-modes` | **Live** viewport display modes (`view.listDisplayModes`) |
| `oblikovati://schema/lighting-styles` | **Live** lighting styles (`lighting.listStyles`) |
| `oblikovati://schema/environment-presets` | **Live** environment presets (`environment.listPresets`) |
| `oblikovati://events/recent` | Recent host events (documents created/saved/activated, commands finished) |

The server's MCP *instructions* point a connecting LLM at these first.

## Host method contract (the automation API)

The contract is the **Apache-2.0 `/api` module** (`oblikovati.org/api`,
[ADR-0018](../../Oblikovati/architecture/decisions/ADR-0018-apache-api-contract-module.md)): the
method-name constants and request/response DTOs live in `api/wire`, and add-ins call
through `api/client` (a typed client over a `Transport` — this bridge backs it with
the host's C-ABI `ObkHostCall`). The MCP tools above are a thin façade over that.
The host side is implemented by `addin/router` in the GPL app repo (`Oblikovati`),
keyed on the same `api/wire`
constants, and is transport-agnostic (today a C-ABI function pointer; a future
gRPC/socket transport could re-front it). The table below mirrors `api/wire`.

| Method | Request | Result |
|--------|---------|--------|
| `commands.list` | `{}` | `{commands:[{id,displayName,tab,category,alias,tooltip,enabled}]}` |
| `commands.execute` | `{id}` | `{ok}` |
| `documents.list` | `{}` | `{documents:[{id,name,type,dirty,visible,active}]}` |
| `documents.create` | `{type,name}` | `{id,name,type,dirty,visible,active}` |
| `documents.activate` | `{id}` | `{ok}` |
| `parameters.list` | `{}` | `{parameters:[{name,kind,expression,value,health?}]}` |
| `parameters.get` | `{name}` | `{name,kind,expression,value,health?}` |
| `parameters.add` | `{name,expression}` | parameter |
| `parameters.set` | `{name,expression}` | parameter |
| `model.tree` | `{}` | `{document,parameters[],sketches,features[{id,name,kind,suppressed,health?}],bodies}` |
| `model.selection` | `{}` | `{count,kinds[]}` |
| `sketch.create` | `{plane}` | `{sketchIndex,plane}` |
| `sketch.rectangle` | `{sketchIndex,width,height}` | `{sketchIndex,profiles}` |
| `features.list` | `{}` | `{kinds:[{kind,summary,schema}]}` |
| `features.add` | `{kind,args}` | descriptor-specific (e.g. extrude → `{feature,kind,bodies}`) |

Feature kinds are self-describing: `features.list` returns each kind's name and a
JSON Schema for its `args`. Today the registry seeds **extrude**
(`{sketchIndex, profileIndex, distance, operation}`); adding a kind is one
`OperationDescriptor` in `addin/opregistry` (in the `Oblikovati` app repo).

## C ABI (for add-in authors / the host loader)

Add-ins are loaded over the C ABI in `oblikovati_addin.h`, which is owned by the
public `oblikovati.org/api` module. `make sync-header` vendors it into `./include`
(git-ignored) before the cgo build. A Go c-shared library runs its **own Go
runtime**, so the boundary is **data-only** (no Go pointers cross). Each add-in
exports:

```c
const char *ObkAddInId(void);                              // stable id
const char *ObkAddInManifest(void);                        // JSON manifest
int   ObkAddInActivate(ObkHostCall call, ObkHostFree free);// start (e.g. MCP server)
int   ObkAddInDeactivate(void);                            // stop
int   ObkAddInNotify(const uint8_t *ev, int len);          // host pushes an event
void  ObkFree(uint8_t *p);                                 // free add-in-owned buffers
```

The host hands the add-in one RPC callback —
`ObkHostCall(method, reqJSON, len, &resp, &respLen)` — plus `ObkHostFree`. The
add-in sends a JSON method request and receives a JSON result. The host runs each
request on its **session goroutine** (a dispatch queue drained once per frame), so
the model is never touched concurrently.

## Layout

```
bridge/        # pure-Go MCP server: tools, resources, events (api/client.Transport-injected, testable)
export.go      # cgo C-ABI exports (ObkAddIn*)
hostcaller.go  # cgo: implements api/client.Transport over the host's ObkHostCall pointer
cmd/mcpcheck/  # tiny streamable-HTTP client to smoke-test a live bridge

# Host side — in the GPL app repo (Oblikovati), pure Go unless noted:
Oblikovati/addin/dispatch      # session-goroutine work queue
Oblikovati/addin/router        # JSON method contract -> *app.Session
Oblikovati/addin/opregistry    # self-describing feature operations (extrude, …)
Oblikovati/addin/events        # session events -> Sink (forwarded to add-ins)
Oblikovati/addin/modelaccess   # active-part helper
Oblikovati/head/internal/addinhost  # cgo loader (dlopen) + //export host-call shim
```

## Limitations / deferred

- **Windows `.dll` loading** is stubbed (clear error) pending the `LoadLibrary`
  trampoline; Linux/macOS (`dlopen`) work.
- **No crash isolation**: in-process, an add-in fault can take the host down
  (accepted for a trusted first-party add-in — ADR-0016).
- Manifest **capabilities/permissions** are recorded but not enforced.
- Sketch geometry is currently `sketch_rectangle`; more primitives
  (`sketch_circle`, lines, polygon) and more feature kinds (revolve, hole, fillet)
  are the natural next descriptors. No `undo`/`delete` tool yet.
