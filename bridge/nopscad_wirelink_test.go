// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopWireLink(t *testing.T) {
	cs := freshPart(t)
	for _, x := range []float64{-0.6, 0.6} {
		s := addSketchOn(t, cs)
		addConstrainedCircle(t, cs, s, []float64{x, 0}, "0.6 mm", "0.6 mm")
		applyFeatureOp(t, cs, s, 0, "12 mm", ternaryOp(x < 0, "new", "join"), "wire leg")
	}
	sTop := addSketchOnPlane(t, cs, "YZ")
	addConstrainedCircle(t, cs, sTop, []float64{0, 0.9}, "0.6 mm", "0.6 mm")
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": sTop, "profileIndex": 0, "distance": "12 mm", "operation": "join", "direction": "symmetric"}); !healthy {
		t.Fatalf("wire top link unhealthy: %s", reason)
	}
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("wire_link volume = %.6f, want positive", got)
	}
}
