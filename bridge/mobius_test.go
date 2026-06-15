// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mobiusSection places one Möbius cross-section: a fixed-frame work plane at azimuth u around a
// ring of radius rCm, its in-plane axes twisted by `twist` (so the band rotates as it goes around),
// then a centered wCm×tCm rectangle on it. Returns the new sketch index. Mirrors cmd/mcpmobius.
//
// The plane frame is (xAxis = width direction, yAxis = thickness direction); since sketch_rectangle
// anchors at the sketch origin, the plane origin is shifted to the band corner so the rectangle is
// centered on the ring. Coordinates are model units (cm); the rectangle size is a unit expression.
func mobiusSection(t *testing.T, cs *mcp.ClientSession, u, twist, rCm, wCm, tCm float64) int {
	t.Helper()
	cu, su := math.Cos(u), math.Sin(u)
	ca, sa := math.Cos(twist), math.Sin(twist)
	wx, wy, wz := ca*cu, ca*su, sa   // width direction (sketch xAxis)
	tx, ty, tz := -sa*cu, -sa*su, ca // thickness direction (sketch yAxis)
	cx, cy := rCm*cu, rCm*su         // section centre on the ring (z=0)
	ox := cx - 0.5*wCm*wx - 0.5*tCm*tx
	oy := cy - 0.5*wCm*wy - 0.5*tCm*ty
	oz := -0.5*wCm*wz - 0.5*tCm*tz

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

	const (
		n             = 24
		rCm, wCm, tCm = 3.0, 1.6, 0.2 // ring radius 30 mm, band 16×2 mm (model units = cm)
	)
	sections := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		u := 2 * math.Pi * float64(i) / float64(n)
		sk := mobiusSection(t, cs, u, u/2, rCm, wCm, tCm) // twist = u/2 → 180° over the loop
		sections = append(sections, map[string]any{"sketchIndex": sk, "profileIndex": 0})
	}

	if healthy, reason := applyFeature(t, cs, "loft", map[string]any{
		"sections": sections, "closed": true, "operation": "new",
	}); !healthy {
		t.Fatalf("closed Möbius loft unhealthy: %s", reason)
	}

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
