// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
	"oblikovati.org/app"
)

// buildBoxTray models a fastened-box base tray (the printed/box.scad family, distilled to the
// shell-and-fasten essence): a W×D×H box hollowed to a wall-thick tray (open top), four corner
// screw bosses on the inner floor, and the top rim rounded. The shell + the boss-on-shelled-
// wall joins + the fillet-on-a-shelled-edge are the boolean family this exercises.
func buildBoxTray(b *partBuilder) {
	for _, p := range [][2]string{
		{"bw", "60 mm"}, {"bd", "40 mm"}, {"bh", "25 mm"}, {"wall", "2 mm"},
		{"boss_d", "8 mm"}, {"boss_h", "20 mm"}, {"hole_d", "3.4 mm"}, {"rim_r", "1.5 mm"},
	} {
		b.param(p[0], p[1])
	}
	cs, t := b.cs, b.t

	// 1. Solid box, corner at origin.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.rectFC(0, [][]float64{{0, 0}, {6, 4}}, "bw", "bd")
	b.feat("1-box", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "bh", "operation": "new"})

	// 2. Hollow to a tray (top face removed).
	b.feat("2-shell", "shell", map[string]any{"faceRefs": []string{topFaceKey(t, cs)}, "thickness": "wall"})

	// 3. A corner screw boss on the inner floor, then 4. replicated to all four corners.
	sBoss := addSketchOn(t, cs)
	b.dim(sBoss, "diameter", "boss_d", b.circle(sBoss, 0.8, 0.8, "0.4 cm")[0])
	b.solved(sBoss)
	bossName := b.feat("3-boss", "extrude", map[string]any{"sketchIndex": sBoss, "profileIndex": 0, "distance": "boss_h", "operation": "join"})
	b.feat("4-boss-pattern", "patternRectangular", map[string]any{
		"sourceFeatures": []string{bossName}, "countX": 2, "countY": 2,
		"stepX": []float64{4.4, 0, 0}, "stepY": []float64{0, 2.4, 0},
	})

	// 5. A screw pilot hole down each boss, patterned.
	sHole := addSketchOn(t, cs)
	b.dim(sHole, "diameter", "hole_d", b.circle(sHole, 0.8, 0.8, "0.17 cm")[0])
	b.solved(sHole)
	holeName := b.feat("5-hole", "extrude", map[string]any{"sketchIndex": sHole, "profileIndex": 0, "extent": "through-all", "operation": "cut"})
	b.feat("6-hole-pattern", "patternRectangular", map[string]any{
		"sourceFeatures": []string{holeName}, "countX": 2, "countY": 2,
		"stepX": []float64{4.4, 0, 0}, "stepY": []float64{0, 2.4, 0},
	})

	// 7. Round the top rim (fillet on the shelled wall's top edges).
	b.feat("7-rim", "fillet", map[string]any{"edgeRefs": []string{topRimEdgeKey(t, cs)}, "radius": "rim_r"})
}

// buildBoxLid models the matching lid: a shallow W×D box hollowed from the bottom (open
// underside) with four corner screw clearance holes through the top.
func buildBoxLid(b *partBuilder) {
	for _, p := range [][2]string{
		{"bw", "60 mm"}, {"bd", "40 mm"}, {"lh", "12 mm"}, {"wall", "2 mm"}, {"hole_d", "3.4 mm"},
	} {
		b.param(p[0], p[1])
	}
	cs, t := b.cs, b.t

	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.rectFC(0, [][]float64{{0, 0}, {6, 4}}, "bw", "bd")
	b.feat("1-box", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "lh", "operation": "new"})

	// Hollow from the bottom (the underside that sits over the tray).
	b.feat("2-shell", "shell", map[string]any{"faceRefs": []string{bottomFaceKey(t, cs)}, "thickness": "wall"})

	// Four corner clearance holes through the lid top.
	sHole := addSketchOn(t, cs)
	for _, xy := range [][2]float64{{0.8, 0.8}, {5.2, 0.8}, {0.8, 3.2}, {5.2, 3.2}} {
		b.dim(sHole, "diameter", "hole_d", b.circle(sHole, xy[0], xy[1], "0.17 cm")[0])
	}
	b.solved(sHole)
	for i := 0; i < 4; i++ {
		b.feat(stepName("3-hole", i), "extrude", map[string]any{"sketchIndex": sHole, "profileIndex": i, "extent": "through-all", "operation": "cut"})
	}
}

// bottomFaceKey returns the active body's lowest planar face — the box's bottom — for shelling.
func bottomFaceKey(t *testing.T, cs *mcp.ClientSession) string {
	return extremeFaceKey(t, cs, 2, false)
}

// TestNopBoxTray builds the base tray; buildBoxTray's feat() validates every step.
func TestNopBoxTray(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}
	buildBoxTray(b)
	if v := partVolume(t, cs); v <= 0 {
		t.Errorf("tray volume = %.4f, want > 0", v)
	}
}

// TestNopBoxLid builds the lid.
func TestNopBoxLid(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}
	buildBoxLid(b)
	if v := partVolume(t, cs); v <= 0 {
		t.Errorf("lid volume = %.4f, want > 0", v)
	}
}

// TestBoxAssembly stacks the lid on the grounded tray with flush constraints on the two outer
// side-wall pairs (X and Y), which align the lid over the tray — the static, fastened-box
// constraint path (vs the jointed PT_camera / hinge assemblies). Asserts the lid's DOF drops.
func TestBoxAssembly(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}

	trayDoc := newPartDoc(t, cs, "box_tray.obk")
	buildBoxTray(b)
	trayX := extremeFaceKey(t, cs, 0, true) // +X outer wall
	trayY := extremeFaceKey(t, cs, 1, true) // +Y outer wall

	lidDoc := newPartDoc(t, cs, "box_lid.obk")
	buildBoxLid(b)
	lidX := extremeFaceKey(t, cs, 0, true)
	lidY := extremeFaceKey(t, cs, 1, true)

	var asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "box_assembly.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	tray := placeComponent(t, cs, "place_component", map[string]any{"document": trayDoc, "name": "tray", "transform": identityCells})
	lid := placeComponent(t, cs, "place_component", map[string]any{"document": lidDoc, "name": "lid", "transform": identityCells})
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": tray, "grounded": true}, nil)

	var added wire.ConstraintResult
	callJSON(t, cs, "add_flush_constraint", map[string]any{"a": geomRef(tray, trayX), "b": geomRef(lid, lidX)}, &added)
	callJSON(t, cs, "add_flush_constraint", map[string]any{"a": geomRef(tray, trayY), "b": geomRef(lid, lidY)}, &added)

	var health wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &health)
	got := dofOf(health, lid)
	if got < 0 || got >= 6 {
		t.Errorf("lid DOF after two flush constraints = %d, want a reduced count (<6)", got)
	}
	t.Logf("box assembly: lid DOF after flushing both outer side-wall pairs = %d", got)
}
