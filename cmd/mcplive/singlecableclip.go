// SPDX-License-Identifier: GPL-2.0-only

package main

func runSingleCableClip(c *caller) error {
	if err := addBoxFeature(c, [][]float64{{-0.8, -0.09}, {0.8, 0.09}}, "16 mm", "1.8 mm", "5 mm", "new"); err != nil {
		return err
	}
	if err := addBoxFeature(c, [][]float64{{-0.2, -0.45}, {0.2, 0.45}}, "4 mm", "9 mm", "5 mm", "new"); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 2, []float64{-0.55, 0.62}, "4.4 mm", "4.4 mm")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 2, "profileIndex": 0, "distance": "5 mm", "operation": "new"}); err != nil {
		return err
	}
	if err := c.applyFeature("hull", map[string]any{}); err != nil {
		return err
	}
	if err := addLiveSlotCut(c, []float64{-0.45, 0.45}, []float64{-0.45, 0.05}, 0.18); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 4, []float64{0.45, 0.45}, "3 mm", "3 mm")
	return c.applyFeature("extrude", map[string]any{"sketchIndex": 4, "profileIndex": 0, "operation": "cut", "extent": "through-all"})
}
