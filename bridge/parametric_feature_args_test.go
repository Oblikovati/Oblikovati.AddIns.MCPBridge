// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestFeatureArgsAreParametric is the cross-feature regression for the rule that
// every feature numeric argument is parameter-aware and recomputes on a parameter
// edit (not frozen at apply time). It builds a box, fillets an edge with the radius
// driven by a parameter, then edits the parameter and asserts the volume changes —
// proving the dress-up argument re-read the parameter on recompute.
func TestFeatureArgsAreParametric(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "fr", "expression": "2 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "20 mm", "height": "20 mm"}, nil)
	if healthy, reason := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"}); !healthy {
		t.Fatalf("extrude unhealthy: %s", reason)
	}

	edges, _ := topology(t, cs)
	if len(edges) == 0 {
		t.Fatal("no edges to fillet")
	}
	// Radius is a parameter reference, not a literal.
	if healthy, reason := applyFeature(t, cs, "fillet",
		map[string]any{"edgeRefs": []string{edges[0]}, "radius": "fr"}); !healthy {
		t.Fatalf("fillet unhealthy: %s", reason)
	}
	before := partVolume(t, cs)

	// Edit the parameter; the fillet must recompute at the new radius.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "fr", "expression": "5 mm"}, nil)
	after := partVolume(t, cs)

	if math.Abs(after-before) < 1e-4 {
		t.Errorf("fillet did not track parameter edit: volume %.5f unchanged after fr 2mm→5mm", before)
	}
}
