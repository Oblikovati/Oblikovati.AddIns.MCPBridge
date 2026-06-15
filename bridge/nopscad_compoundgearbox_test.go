// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/api/wire"
	"oblikovati.org/app"
)

// buildGearboxFrame models the fixed frame of a compound (layshaft) gearbox: a plate carrying
// THREE pivot bosses, at x=0 (input), x=3 cm (the compound idler) and x=6 cm (output). The boss
// top faces are the three gear axes. Spacing is the pitch-radius sum of each mesh (input Ø40 +
// idler Ø20 = 30 mm; idler Ø40 + output Ø20 = 30 mm).
func buildGearboxFrame(b *partBuilder) {
	for _, p := range [][2]string{{"plate_t", "4 mm"}, {"boss_d", "8 mm"}, {"boss_h", "4 mm"}} {
		b.param(p[0], p[1])
	}
	cs, t := b.cs, b.t
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.rectFC(0, [][]float64{{-1.5, -1.5}, {7.5, 1.5}}, "90 mm", "30 mm")
	b.feat("1-plate", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "plate_t", "operation": "new"})
	for i, x := range []float64{0, 3.0, 6.0} {
		sk := addSketchOn(t, cs)
		b.dim(sk, "diameter", "boss_d", b.circle(sk, x, 0, "0.4 cm")[0])
		b.solved(sk)
		b.feat(stepName("2-boss", i), "extrude", map[string]any{"sketchIndex": sk, "profileIndex": 0, "distance": "boss_h", "operation": "join"})
	}
}

// zAngleOf returns the rotation about +Z encoded in a row-major 4×4 placement (atan2 of the
// rotation sub-block), independent of the translation column — so it reads the spin of a gear
// that rotates about an axis offset from the world origin.
func zAngleOf(cells []float64) float64 {
	if len(cells) < 16 {
		return 0
	}
	return math.Atan2(cells[4], cells[0]) // r10, r00
}

// zSwept is the magnitude of the Z-rotation an occurrence accumulated between the first and last
// drive frame.
func zSwept(t *testing.T, res wire.DriveResult, occ uint64) float64 {
	t.Helper()
	first := placementOf(res.Frames[0], occ)
	last := placementOf(res.Frames[len(res.Frames)-1], occ)
	if first == nil || last == nil {
		t.Fatalf("occurrence %d absent from a drive frame", occ)
	}
	return math.Abs(zAngleOf(last) - zAngleOf(first))
}

// TestCompoundGearbox assembles a two-stage (compound / layshaft) reduction and drives it: the
// INPUT gear meshes the lower stage of a COMPOUND IDLER, whose upper stage meshes the OUTPUT
// gear. Both meshes are 2:1, so the train multiplies — output turns 4× the input — and the
// coupling graph is MULTI-HOP (input → idler → output), exercising ratio COMPOSITION across hops
// of the drive's rotate-rotate propagation (Oblikovati/Oblikovati#883), which the single-hop gear
// train (TestGearTrain) does not cover.
//
// Like the gear train this models the kinematics, not the teeth: each gear is a bored blank and
// the idler's two coaxial stages are simplified to one blank (the two 2:1 ratios are set on the
// constraints). Driving the input must turn the idler at 2× and the output at 4× the input.
func TestCompoundGearbox(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}

	frameDoc := newPartDoc(t, cs, "gearbox_frame.obk")
	buildGearboxFrame(b)
	axIn := bossFaceKey(t, cs, 0)    // input pivot
	axIdler := bossFaceKey(t, cs, 3) // compound-idler pivot
	axOut := bossFaceKey(t, cs, 6)   // output pivot

	inputDoc := newPartDoc(t, cs, "gearbox_input.obk")
	buildGearPart(b, "40 mm", "2 cm")
	inputAxis := extremeFaceKey(t, cs, 2, true)

	idlerDoc := newPartDoc(t, cs, "gearbox_idler.obk")
	buildGearPart(b, "40 mm", "2 cm")
	idlerAxis := extremeFaceKey(t, cs, 2, true)

	outputDoc := newPartDoc(t, cs, "gearbox_output.obk")
	buildGearPart(b, "20 mm", "1 cm")
	outputAxis := extremeFaceKey(t, cs, 2, true)

	var asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "gearbox.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	frame := placeComponent(t, cs, "place_component", map[string]any{"document": frameDoc, "name": "frame", "transform": identityCells})
	input := placeComponent(t, cs, "place_component", map[string]any{"document": inputDoc, "name": "input", "transform": identityCells})
	idler := placeComponent(t, cs, "place_component", map[string]any{"document": idlerDoc, "name": "idler", "transform": transformAt(3, 0, 0)})
	output := placeComponent(t, cs, "place_component", map[string]any{"document": outputDoc, "name": "output", "transform": transformAt(6, 0, 0)})
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": frame, "grounded": true}, nil)

	// Each gear spins on the frame (one rotational DOF each).
	var jin wire.AssemblyJointResult
	callJSON(t, cs, "add_rotational_joint", map[string]any{"a": geomRef(frame, axIn), "b": geomRef(input, inputAxis)}, &jin)
	if jin.Joint.DegreesOfFreedom != 1 {
		t.Fatalf("input joint = %d DOF, want 1", jin.Joint.DegreesOfFreedom)
	}
	callJSON(t, cs, "add_rotational_joint", map[string]any{"a": geomRef(frame, axIdler), "b": geomRef(idler, idlerAxis)}, &wire.AssemblyJointResult{})
	callJSON(t, cs, "add_rotational_joint", map[string]any{"a": geomRef(frame, axOut), "b": geomRef(output, outputAxis)}, &wire.AssemblyJointResult{})

	// Two 2:1 meshes through the compound idler.
	var rr wire.ConstraintResult
	callJSON(t, cs, "add_rotate_rotate_constraint", map[string]any{"a": geomRef(input, inputAxis), "b": geomRef(idler, idlerAxis), "ratio": 2.0}, &rr)
	if rr.Constraint.Type != "rotate-rotate" {
		t.Fatalf("mesh 1 constraint = %q, want rotate-rotate", rr.Constraint.Type)
	}
	callJSON(t, cs, "add_rotate_rotate_constraint", map[string]any{"a": geomRef(idler, idlerAxis), "b": geomRef(output, outputAxis), "ratio": 2.0}, &wire.ConstraintResult{})

	callJSON(t, cs, "solve_assembly_constraints", nil, &wire.AssemblyHealthResult{})

	// Drive the input; the ratios must compose along the chain: idler 2×, output 4×.
	span := math.Pi / 4
	var res wire.DriveResult
	callJSON(t, cs, "drive_joint", map[string]any{
		"joint": jin.Joint.ID, "settings": map[string]any{"start": 0.0, "end": span, "step": span / 4},
	}, &res)
	if len(res.Frames) < 2 {
		t.Fatalf("gearbox drive returned %d frames, want a swept range", len(res.Frames))
	}

	inSwept := zSwept(t, res, input)
	idlerSwept := zSwept(t, res, idler)
	outSwept := zSwept(t, res, output)
	if math.Abs(inSwept-span) > 1e-2 {
		t.Errorf("input swept %.4f rad, want %.4f", inSwept, span)
	}
	if math.Abs(idlerSwept-2*span) > 1e-2 {
		t.Errorf("idler swept %.4f rad, want %.4f (2:1 mesh 1)", idlerSwept, 2*span)
	}
	if math.Abs(outSwept-4*span) > 1e-2 {
		t.Errorf("output swept %.4f rad, want %.4f (2:1 × 2:1 composed over two hops)", outSwept, 4*span)
	}
	t.Logf("compound gearbox: input %.4f → idler %.4f → output %.4f rad (1:2:4)", inSwept, idlerSwept, outSwept)
}
