// SPDX-License-Identifier: GPL-2.0-only

package main

import "math"

// runMicroPlate is the live check for Oblikovati#1610 (audit A14): a watch-scale drilled
// plate — 20×20×6 µm with a Ø6 µm bore — must recompute healthy with the analytic volume.
// Before the fix the tessellation path was riddled with cm-anchored absolute gates (the
// 1e-6/1e-5 weld grids, the shared trim-grid tolerance, and turnAngle's DefaultTolerance
// degeneracy guard that tessellated a µm bore as a SQUARE), so the mesh volume came out
// ~27‰ high. The same part is then scaled 1e6× (a 20 m plate) to pin scale invariance.
func runMicroPlate(c *caller) error {
	for _, p := range [][2]string{{"diskR", "0.01 mm"}, {"diskT", "0.006 mm"}, {"boreR", "0.003 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	// Concentric disk + bore, both grounded at the origin, so the 1e6× re-dimension
	// scales the part in place (a corner-grounded rect would grow to one side and
	// leave the bore hanging off the plate).
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "0.01 mm", "diskR")
	if err := c.applyFeature("extrude", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "distance": "diskT", "operation": "new",
	}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 1, []float64{0, 0}, "0.003 mm", "boreR")
	if err := c.applyFeature("extrude", map[string]any{
		"sketchIndex": 1, "profileIndex": 0, "operation": "cut", "extent": "through-all",
	}); err != nil {
		return err
	}
	// Volume in cm³: π(R²−r²)·t at R=1e-3, r=3e-4, t=6e-4 (cm).
	diskVol := func(k float64) float64 {
		R, r, t := 1e-3*k, 3e-4*k, 6e-4*k
		return math.Pi * (R*R - r*r) * t
	}
	// 2% window: the props tessellation inscribes the rims (~1.2% deficit at its default
	// angular quality, identical at every scale) — the check here is that BOTH scales
	// agree with the same analytic value, i.e. no scale-dependent gate fires.
	if err := c.checkVolumeTol("microplate (20 µm)", diskVol(1), 0.02); err != nil {
		return err
	}
	// 1e6×: the same recompute at 20 m — both ends of the scale range must agree.
	for _, p := range [][2]string{{"diskR", "10 m"}, {"diskT", "6 m"}, {"boreR", "3 m"}} {
		c.json("set_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	return c.checkVolumeTol("microplate 1e6x (20 m)", diskVol(1e6), 0.02)
}
