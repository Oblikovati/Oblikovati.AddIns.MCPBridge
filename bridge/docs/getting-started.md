# Driving Oblikovati over MCP

You are connected to a **live, running** Oblikovati instance — a parametric,
history-based CAD application (Inventor-class). Every tool call mutates the running
application's documents, and the changes are visible on screen immediately.

## The loop

1. **Inspect** — call `get_model_tree` to see the active part: its parameters,
   sketches, features, and resulting body count. Call `list_documents` to see what's
   open and which document is active.
2. **Change** — use the parameter and feature tools (below). Each change recomputes
   the model.
3. **Verify** — call `get_model_tree` again and confirm the change took effect.

Always inspect before and after a change.

## Tools

- **Commands**: `list_commands` (every ribbon command + whether it's enabled now),
  `execute_command {id}` (run one — the same effect as clicking it).
- **Documents**: `list_documents`, `create_document {type, name}` where type is
  `part` | `assembly` | `drawing` | `presentation`, `activate_document {id}`.
- **Parameters** (of the active part): `list_parameters`, `get_parameter {name}`,
  `add_parameter {name, expression}`, `set_parameter {name, expression}`.
  Expressions carry **units**, e.g. `"50 mm"`, `"5 cm"`, `"4 cm + 10 mm"`.
- **Geometry**: build a solid from scratch with:
  1. `create_sketch {plane}` — add a sketch on an origin plane (`XY`|`XZ`|`YZ`);
     returns its `sketchIndex`.
  2. `sketch_rectangle {sketchIndex, width, height}` — add a closed rectangle
     profile (sizes as unit expressions, e.g. `"40 mm"`).
  3. `add_feature {kind:"extrude", args:{sketchIndex, profileIndex:0, distance,
     operation}}` — extrude the profile into a solid (`operation`: `new`|`join`|
     `cut`|`intersect`). `list_feature_kinds` returns every feature kind and its
     JSON args schema.
- **Selection**: `get_selection` (read-only for now).
- **Work planes**: `list_work_planes`, `create_work_plane {type, …}` add reference
  geometry to sketch on; `create_work_point {at}` adds a datum point whose ref can
  feed a plane (e.g. three-points) or a redefine re-pick. Each listed user plane self-describes what
  `redefine_work_plane {index, scalars, repick}` can edit on it: its scalars
  (offset distance / angle, with unit and current value) and its re-pickable
  reference slots (plane | axis | point | face).
- **Materials & appearances**: `list_materials` / `list_appearances` (digests with ids),
  `get_material {id}` / `get_appearance {id}` for detail, `assign_material {id}` /
  `assign_appearance {id}` to apply, `get_physical_properties` for mass/volume.
- **Ribbon**: `get_ribbon` reads the active document's ribbon (tabs/panels/controls);
  `create_command {id, displayName, ribbon?, tab?, category?, environment?}` places an
  add-in button on it.
- **Theme**: `get_active_theme`, `list_themes`.

## Reference resources

- `oblikovati://docs/model` — the object model (documents, parameters, sketches,
  features, bodies) in more depth.
- `oblikovati://schema/feature-kinds` — the **live** feature-operation schemas (the
  same data as `list_feature_kinds`); read this before calling `add_feature`.
- `oblikovati://schema/commands` — the live command list.
- `oblikovati://schema/ribbon` — the active document's ribbon structure.
- `oblikovati://schema/work-planes`, `oblikovati://schema/materials`,
  `oblikovati://schema/appearances`, `oblikovati://schema/themes` — live snapshots of
  the active document's reference geometry and asset libraries.

## Notes

- There is one **active document**; the parameter/feature/model tools act on it. Use
  `create_document` or `activate_document` to change which.
- A brand-new part has no features and zero bodies until you add geometry.
- Units: the database length unit is the centimetre, but you always read and write
  values as unit-bearing expressions (e.g. `"50 mm"`), so you never deal with raw
  numbers.
