// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/wire"
)

// TestEndToEndDriveJoint drives a rotational joint through a range over MCP and checks the
// motion frames come back (M12-F03, #366).
func TestEndToEndDriveJoint(t *testing.T) {
	cs, g, f, k := twoBoxes(t)

	var added wire.AssemblyJointResult
	callJSON(t, cs, "add_rotational_joint", map[string]any{"a": geomRef(g, k.edge), "b": geomRef(f, k.edge)}, &added)

	var res wire.DriveResult
	callJSON(t, cs, "drive_joint", map[string]any{
		"joint":    added.Joint.ID,
		"settings": map[string]any{"variable": "angular", "start": 0, "end": 1.0, "step": 0.25},
	}, &res)
	if len(res.Frames) < 2 {
		t.Fatalf("drive frames = %d, want >= 2", len(res.Frames))
	}
	for _, fr := range res.Frames {
		if len(fr.Placements) == 0 {
			t.Errorf("drive frame has no placements: %+v", fr)
		}
	}
}
