// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
	"oblikovati.org/app"
)

// TestPTCameraAssembly composes the ported NopSCADlib PT_camera example (examples/PT_camera):
// two 28BYJ-48 geared steppers and a Pi camera, as separated part documents placed into an
// assembly and joined the way the mechanism moves — a PAN rotational joint (the camera carriage
// stepper rides the base stepper's shaft axis) and a TILT rotational joint (the camera rides the
// carriage stepper's shaft axis). Each rotational joint leaves exactly one rotational DOF, so
// the assembly is fully constrained except for the two intended axes of motion, and each joint
// can be driven through its range. This is the assemble half of the parts-first port: it reuses
// the very part builders the per-part tests validate (buildStepperPart / buildCameraPart).
//
// The joint axis on each faceted part is its lowest planar face — the stepper shaft tip and the
// camera PCB underside — both centred on the part's Z axis with a Z normal, i.e. the shaft /
// mount axis the real mechanism turns about.
func TestPTCameraAssembly(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}

	// --- Two part documents, each modelled once; capture each part's rotation-axis face key. ---
	stepperDoc := newPartDoc(t, cs, "geared_stepper.obk")
	buildStepperPart(b)
	stepperAxis := lowestFaceKey(t, cs)

	cameraDoc := newPartDoc(t, cs, "rpi_camera.obk")
	buildCameraPart(b)
	cameraAxis := lowestFaceKey(t, cs)

	// --- Assembly: place the base stepper (grounded), the carriage stepper, and the camera. ---
	var asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "PT_camera.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	base := placeComponent(t, cs, "place_component", map[string]any{"document": stepperDoc, "name": "base", "transform": identityCells})
	carriage := placeComponent(t, cs, "place_component_copy", map[string]any{"source": base, "name": "carriage", "transform": identityCells})
	camera := placeComponent(t, cs, "place_component", map[string]any{"document": cameraDoc, "name": "camera", "transform": identityCells})
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": base, "grounded": true}, nil)

	// --- PAN joint: carriage stepper turns about the base stepper's shaft axis (1 DOF). ---
	pan := addRotational(t, cs, geomRef(base, stepperAxis), geomRef(carriage, stepperAxis))
	if pan.DegreesOfFreedom != 1 {
		t.Fatalf("pan joint = %d DOF, want 1 (one rotation)", pan.DegreesOfFreedom)
	}

	// --- TILT joint: camera turns about the carriage stepper's shaft axis (1 DOF). ---
	tilt := addRotational(t, cs, geomRef(carriage, stepperAxis), geomRef(camera, cameraAxis))
	if tilt.DegreesOfFreedom != 1 {
		t.Fatalf("tilt joint = %d DOF, want 1 (one rotation)", tilt.DegreesOfFreedom)
	}

	// --- Combined solve: the grounded base fixes the frame; carriage and camera each keep
	// exactly their one intended rotational DOF — the mechanism is otherwise fully constrained. ---
	var health wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &health)
	if got := dofOf(health, carriage); got != 1 {
		t.Errorf("carriage (pan) DOF after solve = %d, want 1", got)
	}
	if got := dofOf(health, camera); got != 1 {
		t.Errorf("camera (tilt) DOF after solve = %d, want 1", got)
	}

	// --- Drive each axis through a quarter turn: every frame must re-solve to a placement. ---
	assertDrivable(t, cs, pan.ID, "pan")
	assertDrivable(t, cs, tilt.ID, "tilt")

	var joints wire.AssemblyJointsResult
	callJSON(t, cs, "list_assembly_joints", nil, &joints)
	if len(joints.Joints) != 2 {
		t.Errorf("assembly joints = %d, want 2 (pan + tilt)", len(joints.Joints))
	}
}

// lowestFaceKey returns the reference key of the active part's lowest planar face (smallest Z of
// the face's representative point) — the shaft-tip / PCB-underside face whose axis is the part's
// rotation axis.
func lowestFaceKey(t *testing.T, cs *mcp.ClientSession) string {
	t.Helper()
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	if len(rk.Bodies) == 0 {
		t.Fatal("get_reference_keys returned no bodies for the rotation-axis face")
	}
	best, bz := "", math.Inf(1)
	for _, f := range rk.Bodies[0].Faces {
		if len(f.Point) == 3 && f.Point[2] < bz {
			best, bz = f.Key, f.Point[2]
		}
	}
	if best == "" {
		t.Fatal("no face carried a representative point for the rotation axis")
	}
	return best
}

// placeComponent places a component (via place_component or place_component_copy) and returns
// the new occurrence id.
func placeComponent(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) uint64 {
	t.Helper()
	var r wire.OccurrenceResult
	callJSON(t, cs, tool, args, &r)
	return r.Occurrence.ID
}

// addRotational adds a rotational joint between two component-geometry refs and returns its info.
func addRotational(t *testing.T, cs *mcp.ClientSession, a, bRef map[string]any) wire.JointInfo {
	t.Helper()
	var added wire.AssemblyJointResult
	callJSON(t, cs, "add_rotational_joint", map[string]any{"a": a, "b": bRef}, &added)
	return added.Joint
}

// assertDrivable drives a joint a quarter turn and fails unless it returns re-solved frames.
func assertDrivable(t *testing.T, cs *mcp.ClientSession, joint uint64, label string) {
	t.Helper()
	var res wire.DriveResult
	callJSON(t, cs, "drive_joint", map[string]any{
		"joint": joint, "settings": map[string]any{"start": 0.0, "end": math.Pi / 2, "step": math.Pi / 8},
	}, &res)
	if len(res.Frames) == 0 {
		t.Errorf("%s joint drive returned no frames", label)
	}
}
