// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestEndToEndFilletCornerRound drives the full MCP → router → feature → kernel stack with the fillet
// cornerType="round" option: filleting the four top edges of a box rounds every top corner into a
// sphere octant (the third edge auto-filleted), so the result carries 4 sphere faces — vs the miter
// default, which leaves a crease and no sphere.
func TestEndToEndFilletCornerRound(t *testing.T) {
	if got := filletTopEdges(t, "round"); got != 4 {
		t.Errorf("cornerType=round: %d sphere faces, want 4 (one octant per top corner)", got)
	}
	if got := filletTopEdges(t, "miter"); got != 0 {
		t.Errorf("cornerType=miter: %d sphere faces, want 0 (a crease, not a sphere)", got)
	}
	// setback rounds each corner into a sphere too (the third edge tapers to a run-out below it).
	if got := filletTopEdges(t, "setback"); got != 4 {
		t.Errorf("cornerType=setback: %d sphere faces, want 4 (a rounded corner per vertex)", got)
	}
}

// filletTopEdges builds a 4x3x2 box, fillets its four top edges with the given cornerType, and
// returns the sphere-face count of the result. It fails the test if the fillet is unhealthy.
func filletTopEdges(t *testing.T, corner string) int {
	t.Helper()
	cs := e2eClient(t, seededSession(t))
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "extrude",
		"args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "2 cm"},
	}, nil)

	var rk refKeys
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	var top []string
	for _, e := range rk.Bodies[0].Edges {
		if len(e.Point) == 3 && math.Abs(e.Point[2]-2) < 1e-6 {
			top = append(top, e.Key)
		}
	}
	if len(top) != 4 {
		t.Fatalf("expected 4 top edges, found %d", len(top))
	}
	var f struct {
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "fillet", "args": map[string]any{"edgeRefs": top, "radius": "3 mm", "cornerType": corner},
	}, &f)
	if !f.Healthy {
		t.Fatalf("cornerType=%s fillet unhealthy: %s", corner, f.Reason)
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	return countFaceKind(rk, "sphere")
}
