// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"

	"oblikovati.org/app"
)

// boxFaceKeysByZ builds a 40×30×20 mm box in the active part and returns the reference keys
// of its top (+Z) and bottom (−Z) faces, picked by their representative points so a mate
// can stack one instance on another deterministically.
func boxFaceKeysByZ(t *testing.T, cs *mcp.ClientSession) (top, bottom string) {
	t.Helper()
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"}, nil)
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "extrude", "args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"},
	}, nil)

	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	if len(rk.Bodies) == 0 || len(rk.Bodies[0].Faces) == 0 {
		t.Fatal("get_reference_keys returned no faces for the box")
	}
	hiZ, loZ := math.Inf(-1), math.Inf(1)
	for _, f := range rk.Bodies[0].Faces {
		if len(f.Point) != 3 {
			continue
		}
		if f.Point[2] > hiZ {
			top, hiZ = f.Key, f.Point[2]
		}
		if f.Point[2] < loZ {
			bottom, loZ = f.Key, f.Point[2]
		}
	}
	if top == "" || bottom == "" {
		t.Fatal("box faces carried no representative points")
	}
	return top, bottom
}

// TestEndToEndAssemblyMateConstraint drives the full M12-F01 surface over MCP: build a box
// part, place two instances in an assembly, ground one, then mate the second's bottom face
// onto the first's top face. The solve must stack the free instance one box-height up (its
// bottom face coincident with the grounded top face), the set must list it, the health must
// report the remaining planar DOF, and delete must clear it — proving the assembly
// constraint tools forward through the bridge → router → solver to the live model.
func TestEndToEndAssemblyMateConstraint(t *testing.T) {
	cs := e2eClient(t, app.NewSession())

	// A part with a box body, whose top/bottom face keys the mate references.
	var widget wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "widget.obk"}, &widget)
	topKey, botKey := boxFaceKeysByZ(t, cs)

	// An assembly holding two instances of the box part.
	var asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "asm.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	var first, second wire.OccurrenceResult
	callJSON(t, cs, "place_component", map[string]any{
		"document": widget.ID, "name": "box:1", "transform": identityCells,
	}, &first)
	callJSON(t, cs, "place_component_copy", map[string]any{
		"source": first.Occurrence.ID, "name": "box:2", "transform": identityCells,
	}, &second)
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": first.Occurrence.ID, "grounded": true}, nil)

	// Mate box:2's bottom face opposed onto the grounded box:1's top face.
	var added wire.ConstraintResult
	callJSON(t, cs, "add_mate_constraint", map[string]any{
		"a":        map[string]any{"occurrence": first.Occurrence.ID, "entity": topKey},
		"b":        map[string]any{"occurrence": second.Occurrence.ID, "entity": botKey},
		"solution": "opposed",
	}, &added)
	if added.Constraint.Type != "mate" {
		t.Fatalf("added constraint type = %q, want mate", added.Constraint.Type)
	}

	// The free instance stacked one box-height (20 mm = 2 cm, the database unit) up.
	var tree wire.OccurrencesResult
	callJSON(t, cs, "list_occurrences", nil, &tree)
	movedZ := occurrenceZ(t, tree, second.Occurrence.ID)
	if math.Abs(movedZ-2.0) > 1e-4 {
		t.Errorf("mated instance Z = %v cm, want ~2.0 (bottom face on the grounded top face)", movedZ)
	}

	// The constraint lists, and the assembly health reports the remaining planar DOF.
	var list wire.ConstraintsResult
	callJSON(t, cs, "list_assembly_constraints", nil, &list)
	if len(list.Constraints) != 1 || list.Constraints[0].ID != added.Constraint.ID {
		t.Fatalf("list = %+v, want the one mate", list.Constraints)
	}

	var health wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &health)
	if health.Status != "under-constrained" || health.DegreesOfFreedom != 3 {
		t.Errorf("health = %+v, want under-constrained with 3 DOF", health)
	}
	if dofOf(health, second.Occurrence.ID) != 3 || dofOf(health, first.Occurrence.ID) != 0 {
		t.Errorf("per-occurrence DOF = %+v, want free=3 grounded=0", health.Occurrences)
	}

	// Deleting the constraint clears the set.
	var afterDel wire.ConstraintsResult
	callJSON(t, cs, "delete_assembly_constraint", map[string]any{"id": added.Constraint.ID}, &afterDel)
	if len(afterDel.Constraints) != 0 {
		t.Errorf("after delete = %+v, want empty set", afterDel.Constraints)
	}
}

// TestEndToEndAssemblyConstraintHealthNoConstraints drives solve/health on an assembly with
// a grounded and a free instance and no constraints: the free box keeps six DOF, the
// grounded box none — the diagnostic an MCP client reads to see what is under-constrained.
func TestEndToEndAssemblyConstraintHealthNoConstraints(t *testing.T) {
	cs := e2eClient(t, app.NewSession())

	var widget, asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "widget.obk"}, &widget)
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "asm.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	var first, second wire.OccurrenceResult
	callJSON(t, cs, "place_component", map[string]any{"document": widget.ID, "name": "a:1", "transform": identityCells}, &first)
	callJSON(t, cs, "place_component_copy", map[string]any{"source": first.Occurrence.ID, "name": "a:2", "transform": identityCells}, &second)
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": first.Occurrence.ID, "grounded": true}, nil)

	var health wire.AssemblyHealthResult
	callJSON(t, cs, "assembly_constraint_health", nil, &health)
	if health.Constraints != 0 {
		t.Errorf("active constraints = %d, want 0", health.Constraints)
	}
	if dofOf(health, second.Occurrence.ID) != 6 || dofOf(health, first.Occurrence.ID) != 0 {
		t.Errorf("per-occurrence DOF = %+v, want free=6 grounded=0", health.Occurrences)
	}
}

// occurrenceZ returns an occurrence's Z translation (cell 11 of its row-major placement)
// from a listed occurrence tree.
func occurrenceZ(t *testing.T, tree wire.OccurrencesResult, id uint64) float64 {
	t.Helper()
	for _, o := range tree.Occurrences {
		if o.ID == id {
			return o.Transform.Cells[11]
		}
	}
	t.Fatalf("occurrence %d not in tree", id)
	return 0
}

// dofOf returns the reported DOF for an occurrence id in a health result.
func dofOf(h wire.AssemblyHealthResult, id uint64) int {
	for _, o := range h.Occurrences {
		if o.Occurrence == id {
			return o.DegreesOfFreedom
		}
	}
	return -1
}
