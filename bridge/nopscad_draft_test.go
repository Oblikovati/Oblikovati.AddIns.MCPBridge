// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// TestNopDraftTaperedBox exercises the DRAFT feature — the mould-taper that OpenSCAD parts
// fake with linear_extrude(scale=…) (printed feet, gridfinity bases, tapered walls). It
// drafts the four side faces of a box about the +Z pull. The sign convention (per the kernel
// draft test) is: NEGATIVE angle tapers the face inward going along pull — the mould-release
// draft — so a box becomes a truncated pyramid that loses material. The volume is exact:
// ∫₀ʰ (a−2z·tan|θ|)² dz = (a³ − (a−2h·tan|θ|)³)/(6·tan|θ|). Draft was previously unexercised by
// the corpus march, so this both covers it and pins the taper geometry/sign.
func TestNopDraftTaperedBox(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	const a, h = 4.0, 2.0 // base side, height (cm)

	// Box a×a×h on XY, extruded +Z.
	s0 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {a / 2, a / 2}}}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s0, "profileIndex": 0, "distance": "20 mm", "operation": "new"})

	// The four SIDE faces: centroid strictly between the bottom (z≈0) and top (z≈h).
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	var sides []string
	for _, f := range rk.Bodies[0].Faces {
		if len(f.Point) == 3 && f.Point[2] > 0.1 && f.Point[2] < h-0.1 {
			sides = append(sides, f.Key)
		}
	}
	if len(sides) != 4 {
		t.Fatalf("want 4 side faces to draft, found %d", len(sides))
	}

	if healthy, reason := applyFeature(t, cs, "draft", map[string]any{"faceRefs": sides, "angle": "-10 deg"}); !healthy {
		t.Fatalf("draft unhealthy: %s", reason)
	}
	part, err := modelaccess.ActivePart(s)
	if err != nil {
		t.Fatalf("active part: %v", err)
	}
	bodies := part.SurfaceBodies().All()
	if len(bodies) != 1 || !ops.Validate(bodies[0]).Valid {
		t.Fatalf("drafted box is not a single valid solid (bodies=%d)", len(bodies))
	}

	tan := math.Tan(10 * math.Pi / 180)
	taperedSide := a - 2*h*tan
	want := (a*a*a - taperedSide*taperedSide*taperedSide) / (6 * tan)
	if got := partVolume(t, cs); math.Abs(got-want)/want > 0.02 {
		t.Errorf("drafted (tapered) box volume = %.6f cm^3, want ~%.6f — draft taper geometry is off", got, want)
	}
	// Sanity: drafting inward must remove material vs the a²h prism.
	if got := partVolume(t, cs); got >= a*a*h {
		t.Errorf("inward draft did not remove material: %.4f >= prism %.4f", got, a*a*h)
	}
}
