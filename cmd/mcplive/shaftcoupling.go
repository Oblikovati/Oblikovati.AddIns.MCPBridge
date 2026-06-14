// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runShaftCoupling is the live driver for a rigid shaft coupling (NopSCADlib
// vitamins/shaft_coupling.scad, SC_5x8_rigid): a faceted cylinder, a stepped through bore,
// and four radial M3 grub-screw holes that pierce the wall into the bore cavity. The grub
// holes are the interesting boolean — a tool partially penetrating the faceted bore wall —
// and they cut cleanly (axis-perpendicular radial holes are the well-behaved case). Shows a
// clean coupling with visible grub holes in the viewport.
func runShaftCoupling(c *caller) error {
	// Solid cylinder Ø12.5 × 25 mm.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "6.25 mm"})
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "25 mm", "operation": "new", "direction": "symmetric"}); err != nil {
		return err
	}
	// Stepped bore: Ø5 lower half (−Z) meeting Ø8 upper half (+Z) at z=0.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.ids(map[string]any{"sketchIndex": 1, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2.5 mm"})
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "distance": "12.5 mm", "operation": "cut", "direction": "negative"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.ids(map[string]any{"sketchIndex": 2, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "4 mm"})
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 2, "profileIndex": 0, "distance": "12.5 mm", "operation": "cut", "direction": "positive"}); err != nil {
		return err
	}
	if err := c.checkVolume("stepped tube", shaftTubeVol()); err != nil {
		return err
	}

	// Four radial M3 grub holes (Ø3) at z=±7.5 mm, along X (YZ sketch) and Y (XZ sketch).
	sk := 3
	for _, g := range []struct {
		plane string
		z     float64
	}{{"YZ", 0.75}, {"XZ", -0.75}, {"YZ", -0.75}, {"XZ", 0.75}} {
		c.json("create_sketch", map[string]any{"plane": g.plane}, nil)
		c.ids(map[string]any{"sketchIndex": sk, "kind": "circle", "points": [][]float64{{0, g.z}}, "radius": "1.5 mm"})
		if err := c.applyFeature("extrude", map[string]any{"sketchIndex": sk, "profileIndex": 0, "distance": "20 mm", "operation": "cut", "direction": "symmetric"}); err != nil {
			return fmt.Errorf("grub hole %d (%s z=%.2f): %w", sk, g.plane, g.z, err)
		}
		sk++
	}
	// Empirical faceted volume after the four grub holes (kernel-faceted circles).
	return c.checkVolume("coupling + 4 grub holes", 1.99988)
}

// shaftTubeVol = outer cylinder minus the stepped bore (cm^3), the analytic ideal; the
// faceted result sits ~1% under, inside checkVolume's band.
func shaftTubeVol() float64 {
	R, r1, r2, half := 6.25/10, 2.5/10, 4.0/10, 12.5/10
	return math.Pi*R*R*(2*half) - math.Pi*r1*r1*half - math.Pi*r2*r2*half
}
