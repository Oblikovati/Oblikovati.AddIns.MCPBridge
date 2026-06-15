// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/app"
)

// TestNopRpiCameraV1 re-models the Raspberry Pi camera V1 (NopSCADlib vitamins/cameras.scad,
// cameras[0] = rpi_camera_v1) as one native, fully-parametric Oblikovati part — the second
// vitamin part of the PT_camera example assembly (the payload the pan/tilt stepper rig aims).
// The feature tree lives in buildCameraPart (shared with the assembly test); this test checks
// it builds one valid solid of the right envelope and volume and rebuilds under a parameter edit.
//
//	PCB extrude(pcb_t, down) → four mounting holes(cut) → lens base block(lens_h, join) →
//	lens barrel(barrel_h, join) → lens top(top_h, join) → aperture bore(cut) → flex connector(join)
//
// The lens is a concentric stepped tower stacked on the PCB top face (8×8 block, Ø7.5 barrel,
// Ø5.5 top) — nested coplanar joins on one plane, the join family the kernel must keep manifold.
//
// rpi_camera_v1 dimensions (mm): PCB 25×24×1 with four Ø2.1 mounting holes at (±2,−2),(±2,9.6);
// lens stack offset (0,−2.4): 8×8×3 base block, Ø7.5×4 barrel, Ø5.5×5 top with a Ø2 aperture;
// flex connector 8×5×1 at (0,8).
func TestNopRpiCameraV1(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}
	buildCameraPart(b)

	// Envelope: PCB span ±12.5 / ±12 mm in X/Y, the lens top at +5 and board bottom at −1 in Z.
	assertEnvelope(t, cs, [3][2]float64{{-1.25, 1.25}, {-1.2, 1.2}, {-0.1, 0.5}})
	if v := partVolume(t, cs); v < 0.6 || v > 1.3 {
		t.Errorf("camera volume = %.4f cm^3, want ~0.9 (0.6–1.3 band)", v)
	}

	// Parametric: a larger PCB must rebuild the whole stack into one valid solid.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "pcb_l", "expression": "30 mm"}, nil)
	b.mustValid("reparam")
}
