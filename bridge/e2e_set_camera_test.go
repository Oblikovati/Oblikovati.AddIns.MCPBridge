// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/app"
)

// TestEndToEndSetCameraAcceptsArrays pins the fix for the set_camera schema trap: its
// eye/target/up are sent as flat [x,y,z] arrays (a types.Point/Vector's JSON form), so the
// tool's input schema must be an array, not the object the SDK derives from the wire struct.
// Before the mcp:input override the validator rejected the array ("type array, want object").
func TestEndToEndSetCameraAcceptsArrays(t *testing.T) {
	cs := e2eClient(t, app.NewSession())
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "view.obk"}, nil)

	var cam struct {
		Eye []float64 `json:"eye"`
		FOV float64   `json:"fov"`
	}
	callJSON(t, cs, "set_camera", map[string]any{
		"eye":    []float64{16, -16, 13},
		"target": []float64{0, 0, 3},
		"up":     []float64{0, 0, 1},
		"fov":    0.6,
	}, &cam)

	if len(cam.Eye) != 3 {
		t.Fatalf("set_camera returned no eye frame: %+v", cam)
	}
	if cam.Eye[0] != 16 || cam.Eye[1] != -16 || cam.Eye[2] != 13 {
		t.Errorf("camera eye = %v, want [16,-16,13]", cam.Eye)
	}
}
