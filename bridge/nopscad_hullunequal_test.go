// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopHullUnequalCylinders stresses the convex-hull op on DISSIMILAR inputs — two
// cylinders of DIFFERENT radii (a tapered standoff / rod-end / lever idiom NopSCADlib uses).
// Unlike the equal-radius stadium (already covered), the hull of two unequal circles is bounded
// by two NON-parallel external tangents, the larger circle contributing a sub-semicircle arc
// and the smaller a super-semicircle arc — geometry that exposes external-tangent bugs in the
// hull. The expected area is exact (closed form below), so a wrong hull shows up as a volume
// error well outside the faceting band.
func TestNopHullUnequalCylinders(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "r1", "expression": "8 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "r2", "expression": "4 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "d", "expression": "12 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "h", "expression": "6 mm"}, nil)

	cyl := func(sk int, rExpr, dExpr string, seedX float64) {
		callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
		c := idsOf(t, cs, map[string]any{"sketchIndex": sk, "kind": "circle", "points": [][]float64{{seedX, 0}}, "radius": "0.4 cm"})
		o := idsOf(t, cs, map[string]any{"sketchIndex": sk, "kind": "point", "points": [][]float64{{0, 0}}})[0]
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sk, "kind": "ground", "entities": []uint64{o}}, nil)
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sk, "kind": "radius", "entities": []uint64{c[0]}, "expression": rExpr}, nil)
		if dExpr == "" {
			callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sk, "kind": "coincident", "entities": []uint64{o, c[1]}}, nil)
		} else {
			callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sk, "kind": "horizontal", "entities": []uint64{o, c[1]}}, nil)
			callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sk, "kind": "distance", "entities": []uint64{o, c[1]}, "expression": dExpr}, nil)
		}
		requireConstrained(t, cs, sk)
		if h, reason := applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": sk, "profileIndex": 0, "distance": "h", "operation": "new"}); !h {
			t.Fatalf("cylinder %d extrude unhealthy: %s", sk, reason)
		}
	}
	cyl(0, "r1", "", 0)    // big cylinder at the origin
	cyl(1, "r2", "d", 1.2) // small cylinder at x=d

	if n := bodyCount(t, cs); n != 2 {
		t.Fatalf("want 2 bodies before hull, got %d", n)
	}
	if h, reason := applyFeature(t, cs, "hull", map[string]any{}); !h {
		t.Fatalf("hull unhealthy: %s", reason)
	}
	if n := bodyCount(t, cs); n != 1 {
		t.Fatalf("hull should leave 1 body, got %d", n)
	}

	if got, w := partVolume(t, cs), unequalHullVol(0.8, 0.4, 1.2, 0.6); math.Abs(got-w)/w > 0.03 {
		t.Errorf("unequal-hull volume = %.6f cm^3, want ~%.6f (3%% faceting band) — external-tangent hull is wrong", got, w)
	}
}

// unequalHullVol is the exact volume of the convex hull of two cylinders (radii r1,r2 cm,
// centre distance d, height h). The 2D hull is the external-tangent trapezoid plus the big
// circle's MAJOR segment and the small circle's MINOR segment beyond their tangent chords
// (verified against a point-sampled ground-truth hull). Reduces to the stadium (πr²+2rd)·h
// when r1==r2.
func unequalHullVol(r1, r2, d, h float64) float64 {
	if r1 < r2 {
		r1, r2 = r2, r1
	}
	g := math.Asin((r1 - r2) / d) // external-tangent tilt; α = π/2−g, so cosα=sin g, sinα=cos g
	sg, cg := math.Sin(g), math.Cos(g)
	bigSegment := r1 * r1 * (math.Pi/2 + g + cg*sg)
	smallSegment := r2 * r2 * (math.Pi/2 - g - cg*sg)
	trapezoid := (r1 + r2) * cg * (d - (r1-r2)*sg)
	return (bigSegment + smallSegment + trapezoid) * h
}
