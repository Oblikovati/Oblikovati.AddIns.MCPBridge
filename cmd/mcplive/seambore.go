// SPDX-License-Identifier: GPL-2.0-only

package main

import "math"

// runSeamBore is the live stress for the Oblikovati#2038 seam re-cut. It is squatrim's disk bored on
// the OTHER axis: an extruded circle pins its analytic cylinder's angle-0 to the sketch +X, so an
// XY-sketched disk carries its seam at +X, and a bore sketched on YZ runs along X — straight through
// that seam. The wall's two exit lenses then STRADDLE the seam; the tessellator used to decline the
// unrolled CDT and fall back to a flat best-fit-plane patch that covers only half the wrap, so the
// part reported 7.1 cm³ against an analytic 30.7 (−77%) on a closed, Validate-clean solid with an
// empty diagnostics array. squatrim itself bores along Y and misses the seam, which is why it never
// caught this. Both bore axes must now measure the same part.
func runSeamBore(c *caller) error {
	for _, p := range [][2]string{{"diskR", "50 mm"}, {"diskH", "4 mm"}, {"rodR", "1.5 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "50 mm", "diskR")
	if err := c.applyFeature("extrude", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "distance": "diskH", "operation": "new", "direction": "symmetric",
	}); err != nil {
		return err
	}
	// YZ sketch ⇒ the tunnel runs along X, through the disk's +X seam.
	c.json("create_sketch", map[string]any{"plane": "YZ"}, nil)
	addConstrainedCircle(c, 1, []float64{0, 0}, "1.5 mm", "rodR")
	if err := c.applyFeature("extrude", map[string]any{
		"sketchIndex": 1, "profileIndex": 0, "operation": "cut",
		"extent": "through-all", "direction": "symmetric",
	}); err != nil {
		return err
	}
	// π·5²·0.4 − π·0.15²·10; OCCT (bcut of the same two cylinders) gives 30.7091 for this solid.
	want := math.Pi*5*5*0.4 - math.Pi*0.15*0.15*10
	if err := c.checkVolumeTol("seambore", want, 0.01); err != nil {
		return err
	}
	// At 20× the seam re-cut has to hold with the SSI seam noise the model-relative stitch tolerates
	// (Oblikovati#1602) — the scale at which the old CSG fallback went non-manifold.
	for _, p := range [][2]string{{"diskR", "1000 mm"}, {"diskH", "80 mm"}, {"rodR", "30 mm"}} {
		c.json("set_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	k := 20.0
	return c.checkVolumeTol("seambore 20x (2 m)",
		math.Pi*(5*k)*(5*k)*(0.4*k)-math.Pi*(0.15*k)*(0.15*k)*(10*k), 0.01)
}
