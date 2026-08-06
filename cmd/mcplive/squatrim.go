// SPDX-License-Identifier: GPL-2.0-only

package main

import "math"

// runSquatRim is the live stress for the Oblikovati#1597 SSI-extent fix: a squat fat disk (R ≫ h)
// with a thin rod hole cut horizontally through its rim is the crossing-cylinder boolean whose SSI
// march used to be sized from the disk's axial height alone — the periodic angular corners of the
// trace window map onto the same generator line, so the corner-to-corner extent missed the 100 mm
// girth entirely. The cut must stay healthy and remove one clean tunnel of material.
func runSquatRim(c *caller) error {
	for _, p := range [][2]string{{"diskR", "50 mm"}, {"diskH", "4 mm"}, {"rodR", "1.5 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	// Disk Ø100 × 4 mm, symmetric about XY so the rim's mid-plane is z=0 in every orientation.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "50 mm", "diskR")
	if err := c.applyFeature("extrude", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "distance": "diskH", "operation": "new", "direction": "symmetric",
	}); err != nil {
		return err
	}
	// Rod tunnel: Ø3 mm through the whole rim at mid-height, normal to XZ (along Y). The sketch plane
	// cuts the disk in half, so the tunnel needs BOTH sides: a through-all extent takes a direction
	// (ExtrudeDefinition.SetThroughAllExtent) and the default kPositiveExtentDirection cuts only the
	// side the sketch normal points to. Asking for the default and then asserting a both-directions
	// volume is what made this driver read 1.13% high and get mis-filed as Oblikovati#2038.
	c.json("create_sketch", map[string]any{"plane": "XZ"}, nil)
	addConstrainedCircle(c, 1, []float64{0, 0}, "1.5 mm", "rodR")
	if err := c.applyFeature("extrude", map[string]any{
		"sketchIndex": 1, "profileIndex": 0, "operation": "cut",
		"extent": "through-all", "direction": "symmetric",
	}); err != nil {
		return err
	}
	// Tunnel ≈ π·rodR²·2·diskR; the (rodR/diskR)² chord correction is far below the tolerance.
	squatVol := func(k float64) float64 { // k = uniform scale factor over the 50/4/1.5 mm baseline
		return math.Pi*(5*k)*(5*k)*(0.4*k) - math.Pi*(0.15*k)*(0.15*k)*(10*k)
	}
	if err := c.checkVolumeTol("squatrim", squatVol(1), 0.01); err != nil {
		return err
	}
	// Scale the whole part 20× (a 2 m disk): the recompute re-runs the crossing-cylinder boolean at
	// an extent where the SSI seam noise dwarfs the retired absolute stitch grid (Oblikovati#1602) —
	// the live check that the model-relative stitch keeps the seam welded at size.
	for _, p := range [][2]string{{"diskR", "1000 mm"}, {"diskH", "80 mm"}, {"rodR", "30 mm"}} {
		c.json("set_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	return c.checkVolumeTol("squatrim 20x (2 m)", squatVol(20), 0.01)
}
