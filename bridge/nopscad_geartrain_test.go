// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/api/wire"
	"oblikovati.org/app"
)

// buildGearFrame models the fixed frame carrying the gear train: a plate with two pivot bosses
// at the gear centres (x=0 and x=3 cm, the 20+10 mm pitch-radius sum). The boss top faces are
// the two gear axes.
func buildGearFrame(b *partBuilder) {
	for _, p := range [][2]string{{"plate_t", "4 mm"}, {"boss_d", "8 mm"}, {"boss_h", "4 mm"}} {
		b.param(p[0], p[1])
	}
	cs, t := b.cs, b.t
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.rectFC(0, [][]float64{{-1.5, -1.5}, {4.5, 1.5}}, "60 mm", "30 mm")
	b.feat("1-plate", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "plate_t", "operation": "new"})
	for i, x := range []float64{0, 3.0} {
		sk := addSketchOn(t, cs)
		b.dim(sk, "diameter", "boss_d", b.circle(sk, x, 0, "0.4 cm")[0])
		b.solved(sk)
		b.feat(stepName("2-boss", i), "extrude", map[string]any{"sketchIndex": sk, "profileIndex": 0, "distance": "boss_h", "operation": "join"})
	}
}

// buildGearPart models a spur gear (simplified to a bored blank — the rotate-rotate constraint,
// not the tooth geometry, is what this exercises): a cylinder of the given pitch diameter with a
// Ø5 bore. Its flat top face is the spin axis.
func buildGearPart(b *partBuilder, diaMM, radCm string) {
	b.param("gear_d", diaMM)
	cs, t := b.cs, b.t
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.dim(0, "diameter", "gear_d", b.circle(0, 0, 0, radCm)[0])
	b.solved(0)
	b.feat("1-blank", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "6 mm", "operation": "new"})
	sBore := addSketchOn(t, cs)
	b.dim(sBore, "diameter", "5 mm", b.circle(sBore, 0, 0, "0.25 cm")[0])
	b.solved(sBore)
	b.feat("2-bore", "extrude", map[string]any{"sketchIndex": sBore, "profileIndex": 0, "extent": "through-all", "operation": "cut"})
}

// TestGearTrain assembles a two-gear train: a grounded frame, a Ø40 driver and a Ø20 driven gear
// each on a rotational joint to the frame (one spin DOF each), coupled by a ROTATE-ROTATE
// constraint at the 2:1 gear ratio — the velocity-coupling relationship (the last common
// assembly relationship not yet exercised). The static structure is asserted (the constraint is
// added and each gear keeps its one spin DOF); driving the driver turns BOTH gears.
//
// The driven gear follows the driver through the ratio: the drive now walks the rotate-rotate
// gear graph and pins each coupled gear's joint at the ratioed angle (Oblikovati/Oblikovati#883),
// so driving one gear turns the whole train.
func TestGearTrain(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}

	frameDoc := newPartDoc(t, cs, "gear_frame.obk")
	buildGearFrame(b)
	axis1 := bossFaceKey(t, cs, 0)   // driver pivot
	axis2 := bossFaceKey(t, cs, 3.0) // driven pivot

	driverDoc := newPartDoc(t, cs, "gear_driver.obk")
	buildGearPart(b, "40 mm", "2 cm")
	driverAxis := extremeFaceKey(t, cs, 2, true)

	drivenDoc := newPartDoc(t, cs, "gear_driven.obk")
	buildGearPart(b, "20 mm", "1 cm")
	drivenAxis := extremeFaceKey(t, cs, 2, true)

	var asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "gear_train.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	frame := placeComponent(t, cs, "place_component", map[string]any{"document": frameDoc, "name": "frame", "transform": identityCells})
	driver := placeComponent(t, cs, "place_component", map[string]any{"document": driverDoc, "name": "driver", "transform": identityCells})
	driven := placeComponent(t, cs, "place_component", map[string]any{"document": drivenDoc, "name": "driven", "transform": identityCells})
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": frame, "grounded": true}, nil)

	// Each gear spins on the frame.
	var jd wire.AssemblyJointResult
	callJSON(t, cs, "add_rotational_joint", map[string]any{"a": geomRef(frame, axis1), "b": geomRef(driver, driverAxis)}, &jd)
	if jd.Joint.DegreesOfFreedom != 1 {
		t.Fatalf("driver joint = %d DOF, want 1", jd.Joint.DegreesOfFreedom)
	}
	callJSON(t, cs, "add_rotational_joint", map[string]any{"a": geomRef(frame, axis2), "b": geomRef(driven, drivenAxis)}, &wire.AssemblyJointResult{})

	// Couple the two spins at the 2:1 ratio.
	var rr wire.ConstraintResult
	callJSON(t, cs, "add_rotate_rotate_constraint", map[string]any{"a": geomRef(driver, driverAxis), "b": geomRef(driven, drivenAxis), "ratio": 2.0}, &rr)
	if rr.Constraint.Type != "rotate-rotate" {
		t.Fatalf("added constraint = %q, want rotate-rotate", rr.Constraint.Type)
	}

	callJSON(t, cs, "solve_assembly_constraints", nil, &wire.AssemblyHealthResult{})

	// Driving the driver must turn the driven gear (its placement changes across the sweep) —
	// the gear ratio in action.
	var res wire.DriveResult
	callJSON(t, cs, "drive_joint", map[string]any{
		"joint": jd.Joint.ID, "settings": map[string]any{"start": 0.0, "end": math.Pi / 2, "step": math.Pi / 8},
	}, &res)
	if len(res.Frames) < 2 {
		t.Fatalf("gear drive returned %d frames, want a swept range", len(res.Frames))
	}
	if !occurrenceMoved(res, driver) {
		t.Errorf("the driver gear did not move when its joint was driven")
	}
	// The driven gear must follow through the 2:1 ratio (Oblikovati/Oblikovati#883): driving the
	// driver propagates through the rotate-rotate coupling to the driven gear's own joint.
	if !occurrenceMoved(res, driven) {
		t.Errorf("the driven gear did not follow when the driver was driven (rotate-rotate ratio not propagated)")
	}
	t.Logf("gear train: driver driven through %d frames; driven gear followed = %v", len(res.Frames), occurrenceMoved(res, driven))
}

// occurrenceMoved reports whether the given occurrence's placement differs between the first and
// last drive frame.
func occurrenceMoved(res wire.DriveResult, occ uint64) bool {
	first := placementOf(res.Frames[0], occ)
	last := placementOf(res.Frames[len(res.Frames)-1], occ)
	if first == nil || last == nil {
		return false
	}
	for i := range first {
		if math.Abs(first[i]-last[i]) > 1e-6 {
			return true
		}
	}
	return false
}

func placementOf(f wire.DriveFrame, occ uint64) []float64 {
	for _, p := range f.Placements {
		if p.Occurrence == occ {
			return p.Transform.Cells[:]
		}
	}
	return nil
}
