# The Oblikovati object model

Oblikovati is a parametric, feature-based CAD modeler. This is the slice the MCP
bridge currently exposes (parts); assemblies, drawings, and presentations exist as
document kinds but their modeling surface is not yet exposed.

## Workspace and documents

The **Workspace** holds the open **Documents** and tracks the **active** one. A
document has a kind (`part`, `assembly`, `drawing`, `presentation`), a name, and a
dirty flag. The bridge's parameter/feature/model tools act on the *active* document,
which must be a **part**.

## Part component definition

A part document's content is a **component definition** containing:

- **Parameters** — named variables driven by **expressions**. An expression is a
  unit-bearing formula like `"50 mm"` or `"width * 2 + 10 mm"` that references other
  parameters by name. Kinds: `user` (you author these), `model` (created by feature
  dimensions), plus read-only `reference`/`derived`. Each parameter reports its
  authored expression, its evaluated value (shown in the document's display units),
  and a health status.
- **Sketches** — 2D geometry (points, lines, arcs, circles, splines) on a plane,
  with constraints. A closed loop of curves forms a **profile**; sketches are
  addressed by index, profiles within a sketch by index.
- **Features** — an ordered **program** of operations (extrude, …) that consumes
  sketches/parameters and builds geometry. This is the "history": editing a parameter
  or sketch and recomputing flows changes through the whole program. Each feature has
  a name, a kind, a suppressed flag, and a health status.
- **Bodies** — the evaluated solid result of running the feature program.

## Work geometry

**Work planes**, **work axes**, and **work points** are datum (reference) geometry:
the part's origin planes plus user-placed datums (offset, three-point, tangent, …)
that host sketches and orient features. `list_work_planes` is self-describing — each
user plane reports its kind plus the inputs `redefine_work_plane {index, scalars,
repick}` accepts: its editable scalars (offset distance / angle, with unit and
current value) and its re-pickable reference slots (plane | axis | point | face).
Redefining a plane recomputes everything built on it.

## How a solid gets built (typical flow)

1. `create_document {type:"part", name:"bracket.obk"}`.
2. `create_sketch {plane:"XY"}` to sketch on an origin (or work) plane, then add a
   closed profile — `sketch_rectangle`, or `add_sketch_entity` for lines/circles/arcs.
3. `add_feature {kind:"extrude", args:{sketchIndex:0, profileIndex:0, distance:"50 mm"}}`
   to turn a closed profile into a solid. `operation` chooses how it combines with
   existing material: `new` | `join` | `cut` | `intersect`.
4. `set_parameter {name:"width", expression:"60 mm"}` to drive a dimension; the model
   recomputes.

## Identity

Documents have session ids (used by `activate_document`). Sketches, profiles, and
features are addressed positionally (by index) or by name in the model tree.
