// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mobiusSection places one Möbius cross-section: a fixed-frame work plane at azimuth u around a
// ring of radius rCm, its in-plane axes twisted by `twist` (so the band rotates as it goes around),
// then a centered 16×2 mm profile on it (a rectangle or an ellipse per `profile`). Returns the new
// sketch index. Mirrors cmd/mcpmobius.
//
// The plane frame is (xAxis = width direction, yAxis = thickness direction). The ellipse is centered
// on the sketch origin, so its plane origin is the section centre; sketch_rectangle anchors at the
// sketch origin, so its plane origin is shifted to the band corner to land centered on the ring.
// Coordinates are model units (cm); sizes are unit-bearing expressions.
func mobiusSection(t *testing.T, cs *mcp.ClientSession, u, twist, rCm, wCm, tCm float64, profile string) int {
	t.Helper()
	cu, su := math.Cos(u), math.Sin(u)
	ca, sa := math.Cos(twist), math.Sin(twist)
	wx, wy, wz := ca*cu, ca*su, sa   // width direction (sketch xAxis)
	tx, ty, tz := -sa*cu, -sa*su, ca // thickness direction (sketch yAxis)
	ox, oy, oz := rCm*cu, rCm*su, 0.0
	if profile == "rect" {
		ox -= 0.5*wCm*wx + 0.5*tCm*tx
		oy -= 0.5*wCm*wy + 0.5*tCm*ty
		oz -= 0.5*wCm*wz + 0.5*tCm*tz
	}
	var wp struct {
		Index   int  `json:"index"`
		Healthy bool `json:"healthy"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{
		"kind":   "fixed-frame",
		"origin": []float64{ox, oy, oz},
		"xaxis":  []float64{wx, wy, wz},
		"yaxis":  []float64{tx, ty, tz},
	}, &wp)
	if !wp.Healthy {
		t.Fatalf("fixed-frame work plane at u=%.3f went unhealthy", u)
	}
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	if profile == "ellipse" {
		callJSON(t, cs, "add_sketch_entity", map[string]any{
			"sketchIndex": sk.SketchIndex, "kind": "ellipse", "points": [][]float64{{0, 0}},
			"axis": []float64{1, 0}, "majorRadius": "8 mm", "minorRadius": "1 mm",
		}, nil)
		return sk.SketchIndex
	}
	var rect struct {
		Profiles int `json:"profiles"`
	}
	callJSON(t, cs, "sketch_rectangle", map[string]any{
		"sketchIndex": sk.SketchIndex, "width": "16 mm", "height": "2 mm",
	}, &rect)
	if rect.Profiles != 1 {
		t.Fatalf("rectangle at u=%.3f formed %d profiles, want 1", u, rect.Profiles)
	}
	return sk.SketchIndex
}

// buildMobius assembles a closed Möbius loft of n `profile` cross-sections (twist = u/2, a 180°
// half-twist over the loop) and returns the closed-loft reason if it went unhealthy.
func buildMobius(t *testing.T, cs *mcp.ClientSession, n int, rCm, wCm, tCm float64, profile string) {
	t.Helper()
	sections := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		u := 2 * math.Pi * float64(i) / float64(n)
		sk := mobiusSection(t, cs, u, u/2, rCm, wCm, tCm, profile)
		sections = append(sections, map[string]any{"sketchIndex": sk, "profileIndex": 0})
	}
	if healthy, reason := applyFeature(t, cs, "loft", map[string]any{
		"sections": sections, "closed": true, "operation": "new",
	}); !healthy {
		t.Fatalf("closed %s Möbius loft unhealthy: %s", profile, reason)
	}
}

// TestE2EMobiusStrip builds a Möbius strip through the public tool surface — N twisting cross-
// sections on fixed-frame work planes, joined by a single CLOSED loft — and is the deterministic
// in-proc companion to cmd/mcpmobius. It guards both loft fixes (2026-06-15):
//   - corner-preserving resample: an elongated rectangle keeps its full area, so the band has the
//     right volume (the bug skinned a 0.5625× cross-section, halving it);
//   - monodromy-aware closure: the 180° half-twist closes seamlessly instead of cramming the twist
//     into the wrap segment (the seam notch).
//
// A thin band of section w×t swept along the ring's centroid (length 2πR) has volume w·t·2πR and
// surface area ≈ 2(w+t)·2πR (one-sided), independent of the twist. Faceting into N straight
// segments shortens the path by <0.5% for N=24, so a 3% tolerance is comfortable.
func TestE2EMobiusStrip(t *testing.T) {
	cs := freshPart(t)
	const n, rCm, wCm, tCm = 24, 3.0, 1.6, 0.2 // ring 30 mm, band 16×2 mm (model units = cm)
	buildMobius(t, cs, n, rCm, wCm, tCm, "rect")

	length := 2 * math.Pi * rCm
	wantVol := wCm * tCm * length // 6.032 cm³
	if v := partVolume(t, cs); math.Abs(v-wantVol)/wantVol > 0.03 {
		t.Errorf("Möbius volume = %.4f cm³, want ≈%.4f (w·t·2πR); a value near %.4f means the resample is cutting corners",
			v, wantVol, 0.5625*wantVol)
	}
	wantArea := 2 * (wCm + tCm) * length // one-sided band surface ≈ 67.86 cm²
	if a := partArea(t, cs); math.Abs(a-wantArea)/wantArea > 0.05 {
		t.Errorf("Möbius area = %.4f cm², want ≈%.4f (2(w+t)·2πR)", a, wantArea)
	}
}

// TestE2EMobiusStripEllipse is the integration guard for the same design with an ELLIPTICAL
// cross-section (a 16×2 mm ellipse via add_sketch_entity). A loft over a curved closed profile must
// still close seamlessly with the right mass: an elliptical band (semi-axes a,b) swept along the
// ring centroid has volume π·a·b·2πR.
func TestE2EMobiusStripEllipse(t *testing.T) {
	cs := freshPart(t)
	const n, rCm, wCm, tCm = 24, 3.0, 1.6, 0.2
	buildMobius(t, cs, n, rCm, wCm, tCm, "ellipse")

	a, b := wCm/2, tCm/2
	wantVol := math.Pi * a * b * 2 * math.Pi * rCm // π·a·b·2πR ≈ 4.74 cm³
	if v := partVolume(t, cs); math.Abs(v-wantVol)/wantVol > 0.05 {
		t.Errorf("elliptical Möbius volume = %.4f cm³, want ≈%.4f (π·a·b·2πR)", v, wantVol)
	}
}
