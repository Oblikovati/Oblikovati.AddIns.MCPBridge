// SPDX-License-Identifier: GPL-2.0-only

package bridge

// instructions is the MCP server's instructions string, shown to a connecting LLM.
// It orients a model with no prior Oblikovati knowledge and points it at the
// self-describing resources/tools. (Richer docs are served as resources in PBI5.)
const instructions = `Oblikovati is a parametric, history-based CAD application (Inventor-class). You are
driving a LIVE instance in real time: tool calls mutate the running app's documents
and the changes are visible immediately.

Model: a Workspace holds Documents. A part Document's content is a component
definition with Parameters (named expressions like "50 mm"), Sketches (2D profiles
on a plane), and a Features program (extrude, etc.) that builds the solid Bodies.

How to work (discover, then act):
- Call get_model_tree to see the active part (parameters, sketches, features, bodies).
- Call list_commands / execute_command to drive ribbon commands.
- Use list_parameters / get_parameter / add_parameter / set_parameter for dimensions;
  expressions carry units, e.g. "50 mm" or "5 cm".
- Features: call list_feature_kinds for the operations and their JSON arg schemas, then
  add_feature with {kind, args} (e.g. extrude a closed profile by sketchIndex/profileIndex
  and distance). create_document / list_documents / activate_document manage documents.
- list_work_planes / create_work_plane add reference geometry to sketch on.
- Materials/appearances: list_/assign_ set the part's physical material and visual style;
  get_physical_properties reads mass/volume.

Sketching (the usual route to a profile):
- 2D: create_sketch (or list_sketches) → author with add_sketch_entity / sketch_rectangle,
  constrain with add_sketch_constraint and add_sketch_dimension, inspect with
  list_sketch_entities / _constraints / _dimensions / _profiles, then solve_sketch. Each
  entity has a session id (from list_sketch_entities) that constraints/dimensions/transform
  reference. Read a tool's input schema for its kind vocabulary (line/circle/arc/rectangle/
  slot/polygon/ellipse/spline + variants; coincident/parallel/tangent/… constraints).
- 3D sketches (sweeps, pipe runs): the sketch3d_* tools mirror the 2D ones in space.

Scene & session (presentation + history, not the geometry recipe):
- View: get_/set_display_mode (visual style), get_/set_shadows.
- Lighting & environment: list_/set_lighting_style, list_lights/add_light/set_light,
  get_/set_environment + load_environment_image (HDR/IBL).
- undo / redo / get_undo_state step the active document's history.
- tail_logs is the real-time operation trace: after a batch of calls, tail it (poll with
  sinceSeq=<previous nextSeq>) to see each command's timing and to catch errors/panics — the
  way to troubleshoot kernel bugs and spot slow operations while driving the app.
- get_reference_keys lists stable face/edge/vertex ids that project_geometry, the 3D-sketch
  include tools, and work planes consume. Add-ins can draw overlays (set_client_graphics)
  and place ribbon buttons (create_command, get_ribbon).

Always inspect with get_model_tree before and after changes; read a tool's input schema (and
list_feature_kinds before add_feature) so you pass the right kind and arguments.`
