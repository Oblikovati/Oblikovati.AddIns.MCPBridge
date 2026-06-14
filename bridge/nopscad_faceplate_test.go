// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopMotorFaceplate models NopSCADlib's motor_faceplate (vitamins/servo_motor.scad) — a
// square plate with a central raised boss, a bore through the boss, and a corner bolt pattern.
// It combines paths in a way worth stressing: an extrude-JOIN boss (raised cylinder), a
// through-all hole drilled into a FEATURE-CREATED face (the boss top — exercises topological
// naming on geometry that did not exist in the base feature), and a cut bolt-hole pattern.
//
// Faithful simplification: plain square plate (NopSCADlib rounds the corners) and a solid boss
// bored through (vs a moulded tube), preserving the structure and giving an exact volume.
//
// Reference: NopSCADlib/vitamins/servo_motor.scad (motor_faceplate: plate + boss + bolt holes).
func TestNopMotorFaceplate(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{{"w", "40 mm"}, {"t", "3 mm"}, {"bossDia", "20 mm"}, {"boreDia", "8 mm"}, {"boltDia", "4 mm"}, {"bossLen", "8 mm"}} {
		callJSON(t, cs, "add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}

	// Plate: a w×w square, corner at the origin, extruded t.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	plate := rectFull(t, cs, [][]float64{{0, 0}, {4, 4}})
	bl, br, tr, tl := plate.points[0], plate.points[1], plate.points[2], plate.points[3]
	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("ground", bl)
	con("horizontal", bl, br)
	con("vertical", bl, tl)
	con("horizontal", tl, tr)
	con("vertical", br, tr)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": []uint64{bl, br}, "expression": "w"}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": []uint64{bl, tl}, "expression": "w"}, nil)
	requireConstrained(t, cs, 0)
	callJSON(t, cs, "add_feature", map[string]any{"kind": "extrude", "args": map[string]any{
		"sketchIndex": 0, "profileIndex": closedProfileIndex(t, cs), "distance": "t", "operation": "new",
	}}, nil)

	// Central boss: a disc at the plate centre, JOINED, standing bossLen tall.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	boss := idsOf(t, cs, map[string]any{"sketchIndex": 1, "kind": "circle", "points": [][]float64{{2, 2}}, "radius": "1 cm"})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 1, "kind": "ground", "entities": []uint64{boss[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 1, "kind": "diameter", "entities": []uint64{boss[0]}, "expression": "bossDia"}, nil)
	requireConstrained(t, cs, 1)
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": 1, "profileIndex": 0, "operation": "join", "distance": "bossLen",
	}); !healthy {
		t.Fatalf("boss unhealthy: %s", reason)
	}

	// Bore through the BOSS TOP face (the now-topmost face, created by the boss feature).
	if healthy, reason := applyFeature(t, cs, "hole", map[string]any{
		"faceRef": topFaceKey(t, cs), "diameter": "boreDia",
	}); !healthy {
		t.Fatalf("central bore unhealthy: %s", reason)
	}

	// Corner bolt holes: a seed cut + a 2×2 pattern (step = w−2·inset = 3 cm).
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	bolt := idsOf(t, cs, map[string]any{"sketchIndex": 2, "kind": "circle", "points": [][]float64{{0.5, 0.5}}, "radius": "0.2 cm"})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 2, "kind": "ground", "entities": []uint64{bolt[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 2, "kind": "diameter", "entities": []uint64{bolt[0]}, "expression": "boltDia"}, nil)
	requireConstrained(t, cs, 2)
	boltName, healthy, reason := addNamedFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": 2, "profileIndex": 0, "operation": "cut", "extent": "through-all",
	})
	if !healthy {
		t.Fatalf("bolt seed unhealthy: %s", reason)
	}
	if healthy, reason := applyFeature(t, cs, "patternRectangular", map[string]any{
		"sourceFeatures": []string{boltName}, "countX": 2, "countY": 2,
		"stepX": []float64{3, 0, 0}, "stepY": []float64{0, 3, 0},
	}); !healthy {
		t.Fatalf("bolt pattern unhealthy: %s", reason)
	}

	if got, w := partVolume(t, cs), faceplateVolume(8); math.Abs(got-w)/w > 0.02 {
		t.Errorf("faceplate volume = %.6f cm^3, want ~%.6f", got, w)
	}
	// Parametric: a taller boss grows the part (bore deepens with it, net adds the ring).
	callJSON(t, cs, "set_parameter", map[string]any{"name": "bossLen", "expression": "12 mm"}, nil)
	if got, w := partVolume(t, cs), faceplateVolume(12); math.Abs(got-w)/w > 0.02 {
		t.Errorf("taller-boss faceplate volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// faceplateVolume = plate + boss material above the plate − central bore (full height) − four
// bolt holes, cm^3 (bossLen in mm; fixed w=40 t=3 bossØ=20 boreØ=8 boltØ=4).
func faceplateVolume(bossLenMM float64) float64 {
	const w, t, bo, bi, rh = 4.0, 0.3, 1.0, 0.4, 0.2
	bl := bossLenMM / 10
	return w*w*t + math.Pi*bo*bo*(bl-t) - math.Pi*bi*bi*bl - 4*math.Pi*rh*rh*t
}
