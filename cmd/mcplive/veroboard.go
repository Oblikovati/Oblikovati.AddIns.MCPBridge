// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runVeroboard is the live driver for a veroboard / perfboard (NopSCADlib
// vitamins/veroboard.scad): a thin substrate drilled with a regular grid of holes via a
// rectangular pattern of one hole cut. It stresses multi-hole-face tessellation and a chain
// of booleans; shows a perfboard in the viewport. (Kept at 6×4 — the chained-boolean cost is
// super-linear, see the in-proc test's timing note.)
func runVeroboard(c *caller) error {
	const holes, strips = 6, 4
	const pitch = 0.254 // cm

	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.ids(map[string]any{"sketchIndex": 0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {1.27, 0.635}}})
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "1.6 mm", "operation": "new"}); err != nil {
		return err
	}
	x0 := -float64(holes-1) / 2 * pitch
	y0 := -float64(strips-1) / 2 * pitch
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.ids(map[string]any{"sketchIndex": 1, "kind": "circle", "points": [][]float64{{x0, y0}}, "radius": "0.5 mm"})
	seed, err := c.applyNamed("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "operation": "cut", "extent": "through-all"})
	if err != nil {
		return err
	}
	if err := c.applyFeature("patternRectangular", map[string]any{
		"sourceFeatures": []string{seed},
		"countX":         holes, "stepX": []float64{pitch, 0, 0},
		"countY": strips, "stepY": []float64{0, pitch, 0},
	}); err != nil {
		return fmt.Errorf("hole-grid pattern: %w", err)
	}

	const L, W, th, r = 2.54, 1.27, 0.16, 0.05
	return c.checkVolume(fmt.Sprintf("veroboard %dx%d", holes, strips), L*W*th-float64(holes*strips)*math.Pi*r*r*th)
}
