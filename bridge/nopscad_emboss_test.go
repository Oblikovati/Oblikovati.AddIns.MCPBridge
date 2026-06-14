// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"os"
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// embossFontPath locates a true-type test face: Arial if this machine has it (not
// redistributable), else Liberation Sans (Arial-metric-compatible; its 'A' has the same
// closed outline + counter hole), or "" to skip.
func embossFontPath() string {
	for _, p := range []string{
		"/home/vmiguel/.steam/debian-installation/steamapps/common/Proton - Experimental/files/share/fonts/arial.ttf",
		"/usr/share/fonts/truetype/msttcorefonts/Arial.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// TestNopEmbossTrueTypeText embosses a real TRUE-TYPE letter onto a plate. The 'A' is a
// sketch TEXT ENTITY (sketch.addText with a font); the emboss references it BY ENTITY ID,
// so the glyph geometry — a closed profile with its counter HOLE — is derived at recompute
// and never baked into the sketch as polylines. The raised volume must equal the plate plus
// the glyph area × depth.
func TestNopEmbossTrueTypeText(t *testing.T) {
	font := embossFontPath()
	if font == "" {
		t.Skip("no Arial/Liberation .ttf found; skipping true-type emboss test")
	}
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	// Plate 2×2 cm, 5 mm thick, below XY (top face at z=0) so raised text sits on top.
	s0 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {1, 1}}}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s0, "profileIndex": 0, "distance": "5 mm", "operation": "new", "direction": "negative"})

	// 'A' as a text entity on XY (10 mm em), anchored to sit on the plate.
	var ts struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, &ts)
	var ent struct {
		EntityID uint64 `json:"entityId"`
	}
	callJSON(t, cs, "add_sketch_text", map[string]any{
		"sketchIndex": ts.SketchIndex, "anchor": []float64{-0.35, -0.35}, "text": "A", "height": "10 mm", "font": font,
	}, &ent)
	if ent.EntityID == 0 {
		t.Fatal("add_sketch_text returned no entity id")
	}

	// The derived 'A' geometry: a closed profile with a hole (its triangular counter).
	area := textGlyphAreaWithHole(t, s, ts.SketchIndex)

	const depth = 0.15 // 1.5 mm raise
	if h, reason := applyFeature(t, cs, "emboss", map[string]any{
		"sketchIndex": ts.SketchIndex, "textEntity": ent.EntityID, "depth": "1.5 mm",
	}); !h {
		t.Fatalf("emboss raise unhealthy: %s", reason)
	}

	part, err := modelaccess.ActivePart(s)
	if err != nil {
		t.Fatalf("active part: %v", err)
	}
	bodies := part.SurfaceBodies().All()
	if len(bodies) != 1 || !ops.Validate(bodies[0]).Valid {
		t.Fatalf("embossed plate is not a single valid solid (bodies=%d)", len(bodies))
	}

	const plate = 2.0 * 2.0 * 0.5 // cm³
	want := plate + area*depth
	if got := partVolume(t, cs); math.Abs(got-want)/want > 0.05 {
		t.Errorf("embossed-'A' volume = %.5f cm^3, want ~%.5f (plate %.3f + glyphArea %.4f × %.3f)", got, want, plate, area, depth)
	}
}

// textGlyphAreaWithHole derives the sketch's first text entity's glyph profiles (the same
// resolver-backed path the emboss recompute uses) and returns their net area, failing the
// test unless the glyph formed a closed profile with at least one counter hole.
func textGlyphAreaWithHole(t *testing.T, s *app.Session, sketchIndex int) float64 {
	t.Helper()
	part, err := modelaccess.ActivePart(s)
	if err != nil {
		t.Fatalf("active part: %v", err)
	}
	sk := part.Sketches().Item(sketchIndex)
	if sk.TextBoxes().Count() == 0 {
		t.Fatal("sketch has no text entity")
	}
	profs, err := sk.TextBoxes().Item(0).TextProfiles(part)
	if err != nil {
		t.Fatalf("derive text profiles: %v", err)
	}
	area, holes := 0.0, 0
	for _, p := range profs {
		area += p.Area()
		holes += len(p.InnerLoops())
	}
	if len(profs) == 0 || holes == 0 {
		t.Fatalf("text glyph derived %d profiles with %d holes, want a closed profile with a counter hole", len(profs), holes)
	}
	return area
}
