// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/wire"
	"oblikovati.org/app"
)

// buildRailPart models a NopSCADlib MGN12 linear rail (vitamins/rails.scad MGN12, simplified):
// a 12×8 mm bar with M3 mounting holes down its centre at 25 mm pitch. The cross-section profile
// (the ball groove) and the counterbores are omitted for v1.
func buildRailPart(b *partBuilder) {
	for _, p := range [][2]string{
		{"rail_w", "12 mm"}, {"rail_h", "8 mm"}, {"rail_len", "100 mm"},
		{"screw_d", "3.5 mm"}, {"screw_pitch", "25 mm"},
	} {
		b.param(p[0], p[1])
	}
	cs, t := b.cs, b.t
	// Bar: length along X (0..100), width along Y (centred), height along Z.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.rectFC(0, [][]float64{{0, -0.6}, {10, 0.6}}, "rail_len", "rail_w")
	b.feat("1-bar", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "rail_h", "operation": "new"})
	// One M3 hole near the end, then a row of them along the rail (pitch 25 mm).
	sHole := addSketchOn(t, cs)
	b.dim(sHole, "diameter", "screw_d", b.circle(sHole, 0.75, 0, "0.175 cm")[0])
	b.solved(sHole)
	hole := b.feat("2-hole", "extrude", map[string]any{"sketchIndex": sHole, "profileIndex": 0, "extent": "through-all", "operation": "cut"})
	b.feat("3-hole-row", "patternRectangular", map[string]any{
		"sourceFeatures": []string{hole}, "countX": 4, "countY": 1,
		"stepX": []float64{2.5, 0, 0}, "stepY": []float64{0, 1, 0},
	})
}

// buildCarriagePart models the MGN12C carriage (simplified): a 34.7×27×13 mm block with four
// M3 mounting holes (15×20 mm pitch) through the top. The recirculating-ball channel underneath
// is omitted for v1; the slider joint provides the rail motion.
func buildCarriagePart(b *partBuilder) {
	for _, p := range [][2]string{
		{"car_len", "34.7 mm"}, {"car_w", "27 mm"}, {"car_h", "13 mm"},
		{"screw_d", "3.5 mm"}, {"pitch_x", "15 mm"}, {"pitch_y", "20 mm"},
	} {
		b.param(p[0], p[1])
	}
	cs, t := b.cs, b.t
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.rectFC(0, [][]float64{{0, -1.35}, {3.47, 1.35}}, "car_len", "car_w")
	b.feat("1-block", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "car_h", "operation": "new"})
	// Four mounting holes at (±pitch_x/2, ±pitch_y/2) about the block centre (1.735, 0).
	sHole := addSketchOn(t, cs)
	for _, xy := range [][2]float64{{0.985, -1.0}, {2.485, -1.0}, {0.985, 1.0}, {2.485, 1.0}} {
		b.dim(sHole, "diameter", "screw_d", b.circle(sHole, xy[0], xy[1], "0.175 cm")[0])
	}
	b.solved(sHole)
	for i := 0; i < 4; i++ {
		b.feat(stepName("2-hole", i), "extrude", map[string]any{"sketchIndex": sHole, "profileIndex": i, "extent": "through-all", "operation": "cut"})
	}
}

// TestNopMGN12Rail builds the rail part.
func TestNopMGN12Rail(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)
	buildRailPart(&partBuilder{t: t, s: s, cs: cs})
	if v := partVolume(t, cs); v <= 0 {
		t.Errorf("rail volume = %.4f, want > 0", v)
	}
}

// TestNopMGN12Carriage builds the carriage part.
func TestNopMGN12Carriage(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)
	buildCarriagePart(&partBuilder{t: t, s: s, cs: cs})
	if v := partVolume(t, cs); v <= 0 {
		t.Errorf("carriage volume = %.4f, want > 0", v)
	}
}

// TestMGN12LinearGuideAssembly assembles the rail (grounded) and the carriage with a SLIDER
// joint — the carriage keeps exactly one translational degree of freedom (the slide along the
// rail), which the drive sweeps. This is the prismatic-mechanism counterpart to the rotational
// PT_camera/hinge assemblies.
func TestMGN12LinearGuideAssembly(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}

	railDoc := newPartDoc(t, cs, "mgn12_rail.obk")
	buildRailPart(b)
	railTop := extremeFaceKey(t, cs, 2, true) // rail top face (normal +Z)

	carDoc := newPartDoc(t, cs, "mgn12_carriage.obk")
	buildCarriagePart(b)
	carBottom := extremeFaceKey(t, cs, 2, false) // carriage underside (normal −Z)

	var asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "mgn12_guide.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	rail := placeComponent(t, cs, "place_component", map[string]any{"document": railDoc, "name": "rail", "transform": identityCells})
	car := placeComponent(t, cs, "place_component", map[string]any{"document": carDoc, "name": "carriage", "transform": identityCells})
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": rail, "grounded": true}, nil)

	var added wire.AssemblyJointResult
	callJSON(t, cs, "add_slider_joint", map[string]any{"a": geomRef(rail, railTop), "b": geomRef(car, carBottom)}, &added)
	if added.Joint.DegreesOfFreedom != 1 {
		t.Fatalf("slider joint = %d DOF, want 1 (the carriage's slide)", added.Joint.DegreesOfFreedom)
	}

	var health wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &health)
	if got := dofOf(health, car); got != 1 {
		t.Errorf("carriage DOF after solve = %d, want 1 (one translation)", got)
	}
	assertDrivable(t, cs, added.Joint.ID, "carriage-slide")
}
