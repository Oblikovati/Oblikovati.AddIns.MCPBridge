// SPDX-License-Identifier: GPL-2.0-only

package main

import "math"

// runHullUnequal is the live driver for the convex hull of two DIFFERENT-radius cylinders (a
// tapered standoff / rod-end / lever idiom): the hull is bounded by two non-parallel external
// tangents, the big circle contributing a major-segment cap and the small a minor-segment cap.
// Shows a tapered "stadium" prism in the viewport and checks the exact hull volume.
func runHullUnequal(c *caller) error {
	cyl := func(sk int, seedX float64, radius string) error {
		c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
		c.ids(map[string]any{"sketchIndex": sk, "kind": "circle", "points": [][]float64{{seedX, 0}}, "radius": radius})
		return c.applyFeature("extrude", map[string]any{"sketchIndex": sk, "profileIndex": 0, "distance": "6 mm", "operation": "new"})
	}
	if err := cyl(0, 0, "8 mm"); err != nil { // big cylinder r=8 mm at origin
		return err
	}
	if err := cyl(1, 1.2, "4 mm"); err != nil { // small cylinder r=4 mm at x=12 mm
		return err
	}
	if err := c.applyFeature("hull", map[string]any{}); err != nil {
		return err
	}
	return c.checkVolume("tapered hull (r1=8,r2=4,d=12)", taperedHullVol(0.8, 0.4, 1.2, 0.6))
}

// taperedHullVol = convex hull of two cylinders (trapezoid + big major segment + small minor
// segment, extruded). See the bridge test's unequalHullVol for the derivation.
func taperedHullVol(r1, r2, d, h float64) float64 {
	if r1 < r2 {
		r1, r2 = r2, r1
	}
	g := math.Asin((r1 - r2) / d)
	sg, cg := math.Sin(g), math.Cos(g)
	return (r1*r1*(math.Pi/2+g+cg*sg) + r2*r2*(math.Pi/2-g-cg*sg) + (r1+r2)*cg*(d-(r1-r2)*sg)) * h
}
