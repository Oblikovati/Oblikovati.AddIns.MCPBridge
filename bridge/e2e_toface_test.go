// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"strings"
	"testing"
)

// refTopology is the get_reference_keys shape with representative points, so the test can pick a
// body's top face by its z height.
type refTopology struct {
	Bodies []struct {
		Faces []struct {
			Key   string    `json:"key"`
			Point []float64 `json:"point"`
		} `json:"faces"`
	} `json:"bodies"`
}

func topFaceKeyAtZ(faces []struct {
	Key   string    `json:"key"`
	Point []float64 `json:"point"`
}, minZ float64) string {
	for _, f := range faces {
		if len(f.Point) == 3 && f.Point[2] >= minZ {
			return f.Key
		}
	}
	return ""
}

func maxFaceZ(faces []struct {
	Key   string    `json:"key"`
	Point []float64 `json:"point"`
}) float64 {
	max := -1e18
	for _, f := range faces {
		if len(f.Point) == 3 && f.Point[2] > max {
			max = f.Point[2]
		}
	}
	return max
}

// TestExtrudeToFaceOverBridge live-confirms Extrude "To Face" end to end over the MCP bridge: it
// drives the real MCP client → generated tools → wire → router → kernel, with no in-process
// shortcut. It also asserts the bridge advertises the to-face extent in the extrude kind's schema
// (how an add-in/LLM discovers it), then extrudes a profile up to a box's top face and verifies the
// new body terminates there.
func TestExtrudeToFaceOverBridge(t *testing.T) {
	cs := boxClient(t) // active part: one 40×30×20 mm box, top face at z = 2 cm

	// 1. The bridge advertises to-face in the extrude schema (the MCP discovery path).
	var kinds struct {
		Kinds []struct {
			Kind   string          `json:"kind"`
			Schema json.RawMessage `json:"schema"`
		} `json:"kinds"`
	}
	callJSON(t, cs, "list_feature_kinds", nil, &kinds)
	var extrudeSchema string
	for _, k := range kinds.Kinds {
		if k.Kind == "extrude" {
			extrudeSchema = string(k.Schema)
		}
	}
	if extrudeSchema == "" {
		t.Fatal("list_feature_kinds did not advertise the extrude kind over the bridge")
	}
	if !strings.Contains(extrudeSchema, "to-face") || !strings.Contains(extrudeSchema, "toFace") {
		t.Fatalf("the bridge's extrude schema does not advertise the to-face extent: %s", extrudeSchema)
	}

	// 2. Find the box's top face key.
	var rk refTopology
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	if len(rk.Bodies) == 0 {
		t.Fatal("get_reference_keys returned no bodies")
	}
	top := topFaceKeyAtZ(rk.Bodies[0].Faces, 1.9)
	if top == "" {
		t.Fatal("no top face at z≈2 cm on the box")
	}

	// 3. Extrude a second profile "to face" up to that top face — the new capability, over MCP.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 1, "width": "10 mm", "height": "10 mm"}, nil)
	var res struct {
		Bodies  int    `json:"bodies"`
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "extrude",
		"args": map[string]any{
			"sketchIndex": 1, "profileIndex": 0, "extent": "to-face", "toFace": top, "operation": "new",
		},
	}, &res)
	if !res.Healthy {
		t.Fatalf("to-face extrude over bridge unhealthy: %s", res.Reason)
	}
	if res.Bodies != 2 {
		t.Fatalf("bodies = %d, want 2 (base + to-face body)", res.Bodies)
	}

	// 4. Verify the new body reached the target plane (top face z ≈ 2 cm).
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	z := maxFaceZ(rk.Bodies[1].Faces)
	if z < 1.99 || z > 2.01 {
		t.Fatalf("to-face body top z = %.4f cm, want ~2 (terminated at the picked face)", z)
	}
	t.Logf("LIVE OK: Extrude To-Face over MCP bridge → 2 bodies; new body terminates at z=%.3f cm (target face)", z)
}
