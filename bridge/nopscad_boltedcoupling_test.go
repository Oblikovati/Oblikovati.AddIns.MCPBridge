// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/wire"
	"oblikovati.org/app"
)

// buildFlangePart models a bolted-coupling flange (the NopSCADlib flanged-coupling family,
// vitamins/shaft_coupling.scad / leadnut bolt-circle): a disc with a central bore and a ring of
// bolt holes on a bolt circle (a feature-level circular pattern). Two of these bolt face to
// face into a coupling.
func buildFlangePart(b *partBuilder) {
	for _, p := range [][2]string{
		{"disc_d", "60 mm"}, {"disc_t", "8 mm"}, {"bore_d", "20 mm"},
		{"bolt_d", "6 mm"}, {"bolt_r", "22.5 mm"},
	} {
		b.param(p[0], p[1])
	}
	cs, t := b.cs, b.t

	// 1. Disc + 2. central bore (one annular solid).
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.dim(0, "diameter", "disc_d", b.circle(0, 0, 0, "3 cm")[0])
	b.solved(0)
	b.feat("1-disc", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "disc_t", "operation": "new"})

	sBore := addSketchOn(t, cs)
	b.dim(sBore, "diameter", "bore_d", b.circle(sBore, 0, 0, "1 cm")[0])
	b.solved(sBore)
	b.feat("2-bore", "extrude", map[string]any{"sketchIndex": sBore, "profileIndex": 0, "extent": "through-all", "operation": "cut"})

	// 3. One bolt hole on the bolt circle, then 4. a 6-up circular pattern of it.
	sHole := addSketchOn(t, cs)
	o := idsOf(t, cs, map[string]any{"sketchIndex": sHole, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	hole := idsOf(t, cs, map[string]any{"sketchIndex": sHole, "kind": "circle", "points": [][]float64{{2.25, 0}}, "radius": "0.3 cm"})
	b.con(sHole, "ground", o)
	b.con(sHole, "horizontal", o, hole[1])
	b.dim(sHole, "distance", "bolt_r", o, hole[1])
	b.dim(sHole, "diameter", "bolt_d", hole[0])
	b.solved(sHole)
	holeName := b.feat("3-bolthole", "extrude", map[string]any{"sketchIndex": sHole, "profileIndex": 0, "extent": "through-all", "operation": "cut"})
	b.feat("4-bolt-pattern", "patternCircular", map[string]any{
		"sourceFeatures": []string{holeName}, "count": 6, "angle": "360 deg", "axisDir": []float64{0, 0, 1},
	})
}

// TestNopFlange builds one coupling flange.
func TestNopFlange(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)
	buildFlangePart(&partBuilder{t: t, s: s, cs: cs})
	if v := partVolume(t, cs); v <= 0 {
		t.Errorf("flange volume = %.4f, want > 0", v)
	}
}

// TestBoltedCouplingInsert assembles two flanges face to face with an INSERT constraint on
// their mating faces — the canonical "bolt into a hole" relationship: the insert makes the
// flange axes collinear and seats the faces together (removing five DOF), leaving the coupling
// free to spin about the common axis (one DOF). That is exactly the insert constraint's defining
// behaviour; a tightened/keyed coupling would lock the final spin.
func TestBoltedCouplingInsert(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}

	flangeDoc := newPartDoc(t, cs, "flange.obk")
	buildFlangePart(b)
	topFace := extremeFaceKey(t, cs, 2, true)  // +Z mating face
	botFace := extremeFaceKey(t, cs, 2, false) // −Z mating face

	var asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "coupling.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	a := placeComponent(t, cs, "place_component", map[string]any{"document": flangeDoc, "name": "flangeA", "transform": identityCells})
	bb := placeComponent(t, cs, "place_component_copy", map[string]any{"source": a, "name": "flangeB", "transform": identityCells})
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": a, "grounded": true}, nil)

	// Insert flange A's top face into flange B's bottom face (opposed → the faces seat together).
	var added wire.ConstraintResult
	callJSON(t, cs, "add_insert_constraint", map[string]any{"a": geomRef(a, topFace), "b": geomRef(bb, botFace)}, &added)
	if added.Constraint.Type != "insert" {
		t.Fatalf("added constraint = %q, want insert", added.Constraint.Type)
	}

	var health wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &health)
	if got := dofOf(health, bb); got != 1 {
		t.Errorf("flange B DOF after insert = %d, want 1 (coaxial + seated, free to spin)", got)
	}
}
