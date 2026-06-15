// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
	"oblikovati.org/app"
)

// buildHingeLeaf models one leaf of the NopSCADlib flat hinge (printed/flat_hinge.scad,
// small_hinge: 20×16×2, knuckle Ø7, pin Ø2.85, clearance 0.2) into the active part: a flat
// leaf plate with two screw holes, plus the leaf's knuckles — Ø7 cylinders running along the X
// (hinge) axis at the plate's hinge edge, bored Ø2.85 for the pin. `spans` are the [x0,x1] cm
// extents of this leaf's knuckles along the axis (the male leaf owns the two outer knuckles, the
// female the one in the middle, so interleaved they share the axis with the clearance gap).
//
// The knuckle-onto-plate-edge unions are tangent/coplanar contacts on a faceted cylinder — the
// boolean family this porting effort stresses. The far-corner leaf rounding and the teardrop
// (3D-print) knuckle profile are simplified to square corners and plain cylinders for v1.
func buildHingeLeaf(b *partBuilder, spans [][2]float64) {
	for _, p := range [][2]string{
		{"hw", "20 mm"}, {"hd", "16 mm"}, {"ht", "2 mm"}, {"kd", "7 mm"}, {"pd", "2.85 mm"},
		{"screw_d", "3.4 mm"}, {"knuckle_z", "3.5 mm"},
	} {
		b.param(p[0], p[1])
	}
	cs, t := b.cs, b.t

	// 1. Leaf plate: hw×hd, hinge edge at y=0, extrude up ht.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.rectFC(0, [][]float64{{-1.0, 0}, {1.0, 1.6}}, "hw", "hd")
	b.feat("1-plate", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "ht", "operation": "new"})

	// 2. Two M3 screw holes (staggered, like hinge_screw_positions), cut through the plate.
	sHoles := addSketchOn(t, cs)
	b.dim(sHoles, "diameter", "screw_d", b.circle(sHoles, -0.55, 0.45, "0.17 cm")[0])
	b.dim(sHoles, "diameter", "screw_d", b.circle(sHoles, 0.55, 1.15, "0.17 cm")[0])
	b.solved(sHoles)
	b.feat("2-holeA", "extrude", map[string]any{"sketchIndex": sHoles, "profileIndex": 0, "distance": "ht", "operation": "cut"})
	b.feat("2-holeB", "extrude", map[string]any{"sketchIndex": sHoles, "profileIndex": 1, "distance": "ht", "operation": "cut"})

	// 3. Knuckles: Ø7 cylinders along X at the hinge edge (y=0, z=knuckle_z), join to the plate.
	for i, sp := range spans {
		wp := b.workPlane("origin/plane/yz", fmt.Sprintf("%g mm", sp[0]*10))
		si := b.sketchOn(wp)
		b.dim(si, "diameter", "kd", b.circle(si, 0, 0.35, "0.35 cm")[0])
		b.solved(si)
		width := fmt.Sprintf("%g mm", (sp[1]-sp[0])*10)
		b.feat(stepName("3-knuckle", i), "extrude", map[string]any{"sketchIndex": si, "profileIndex": 0, "distance": width, "operation": "join"})
	}

	// 4. Pin bore: Ø2.85 along the full X width through the knuckles, cut.
	wp := b.workPlane("origin/plane/yz", "-10 mm")
	si := b.sketchOn(wp)
	b.dim(si, "diameter", "pd", b.circle(si, 0, 0.35, "0.1425 cm")[0])
	b.solved(si)
	b.feat("4-bore", "extrude", map[string]any{"sketchIndex": si, "profileIndex": 0, "distance": "hw", "operation": "cut"})
}

// maleKnuckleSpans / femaleKnuckleSpans are the X extents (cm) of the interleaved knuckles for
// the small_hinge (male knuckle width 4.9, female 9.8, clearance 0.2, over the 20 mm width).
var (
	maleKnuckleSpans   = [][2]float64{{-1.0, -0.51}, {0.51, 1.0}}
	femaleKnuckleSpans = [][2]float64{{-0.49, 0.49}}
)

// TestNopHingeMaleLeaf builds the male hinge leaf (two outer knuckles) as one valid solid.
func TestNopHingeMaleLeaf(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}
	buildHingeLeaf(b, maleKnuckleSpans)

	// Envelope: width ±10, depth 0..16, and the knuckle Ø7 lifts Z to 7 mm and Y to −3.5 (mm).
	assertEnvelope(t, cs, [3][2]float64{{-1.0, 1.0}, {-0.35, 1.6}, {0, 0.7}})
}

// TestNopHingeFemaleLeaf builds the female hinge leaf (one middle knuckle) as one valid solid.
func TestNopHingeFemaleLeaf(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}
	buildHingeLeaf(b, femaleKnuckleSpans)
	assertEnvelope(t, cs, [3][2]float64{{-1.0, 1.0}, {-0.35, 1.6}, {0, 0.7}})
}

// buildPinPart models the hinge pin: a Ø2.85 × 22 mm cylinder along the X (hinge) axis.
func buildPinPart(b *partBuilder) {
	b.param("pd", "2.85 mm")
	b.param("pin_len", "22 mm")
	wp := b.workPlane("origin/plane/yz", "-11 mm")
	si := b.sketchOn(wp)
	b.dim(si, "diameter", "pd", b.circle(si, 0, 0, "0.1425 cm")[0])
	b.solved(si)
	b.feat("pin", "extrude", map[string]any{"sketchIndex": si, "profileIndex": 0, "distance": "pin_len", "operation": "new"})
}

// TestFlatHingeAssembly composes the ported flat hinge: a male leaf (grounded), a female leaf
// joined to it by a ROTATIONAL joint about the pin (X) axis — the one intended DOF, the hinge
// swing — and the pin RIGID to the male leaf. The joint axis on each part is its outermost
// knuckle end face (normal X, centred on the pin axis). Asserts the female keeps exactly one
// rotational DOF, the pin none, and the swing drives.
func TestFlatHingeAssembly(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}

	maleDoc := newPartDoc(t, cs, "hinge_male.obk")
	buildHingeLeaf(b, maleKnuckleSpans)
	maleAxis := extremeFaceKey(t, cs, 0, false) // leftmost knuckle end, normal X

	femaleDoc := newPartDoc(t, cs, "hinge_female.obk")
	buildHingeLeaf(b, femaleKnuckleSpans)
	femaleAxis := extremeFaceKey(t, cs, 0, false)

	pinDoc := newPartDoc(t, cs, "hinge_pin.obk")
	buildPinPart(b)
	pinAxis := extremeFaceKey(t, cs, 0, false)

	var asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "flat_hinge.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	male := placeComponent(t, cs, "place_component", map[string]any{"document": maleDoc, "name": "male", "transform": identityCells})
	female := placeComponent(t, cs, "place_component", map[string]any{"document": femaleDoc, "name": "female", "transform": identityCells})
	pin := placeComponent(t, cs, "place_component", map[string]any{"document": pinDoc, "name": "pin", "transform": identityCells})
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": male, "grounded": true}, nil)

	swing := addRotational(t, cs, geomRef(male, maleAxis), geomRef(female, femaleAxis))
	if swing.DegreesOfFreedom != 1 {
		t.Fatalf("hinge swing joint = %d DOF, want 1 (one rotation about the pin)", swing.DegreesOfFreedom)
	}
	var rigid wire.AssemblyJointResult
	callJSON(t, cs, "add_rigid_joint", map[string]any{"a": geomRef(male, maleAxis), "b": geomRef(pin, pinAxis)}, &rigid)

	var health wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &health)
	if got := dofOf(health, female); got != 1 {
		t.Errorf("female leaf DOF after solve = %d, want 1 (the hinge swing)", got)
	}
	if got := dofOf(health, pin); got != 0 {
		t.Errorf("pin DOF after solve = %d, want 0 (rigid to the male leaf)", got)
	}
	assertDrivable(t, cs, swing.ID, "hinge-swing")
}

// extremeFaceKey returns the reference key of the active part's face whose representative point
// is the smallest (wantMax=false) or largest (wantMax=true) along the given axis (0=X,1=Y,2=Z).
func extremeFaceKey(t *testing.T, cs *mcp.ClientSession, axis int, wantMax bool) string {
	t.Helper()
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	best, bestV := "", math.Inf(1)
	if wantMax {
		bestV = math.Inf(-1)
	}
	for _, f := range rk.Bodies[0].Faces {
		if len(f.Point) != 3 {
			continue
		}
		if (wantMax && f.Point[axis] > bestV) || (!wantMax && f.Point[axis] < bestV) {
			best, bestV = f.Key, f.Point[axis]
		}
	}
	if best == "" {
		t.Fatalf("no face carried a representative point along axis %d", axis)
	}
	return best
}
