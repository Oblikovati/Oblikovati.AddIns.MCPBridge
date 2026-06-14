// SPDX-License-Identifier: GPL-2.0-only

package main

func runExtrusionCenterSection(c *caller) error {
	if err := addBoxFeature(c, [][]float64{{-0.1, -1}, {0.1, 1}}, "2 mm", "20 mm", "1.2 mm", "new"); err != nil {
		return err
	}
	if err := addBoxFeature(c, [][]float64{{-1, -0.1}, {1, 0.1}}, "20 mm", "2 mm", "1.2 mm", "join"); err != nil {
		return err
	}
	for _, side := range []float64{-1, 1} {
		if err := addBoxFeature(c, [][]float64{{side*0.72 - 0.09, -0.55}, {side*0.72 + 0.09, 0.55}}, "1.8 mm", "11 mm", "1.2 mm", "join"); err != nil {
			return err
		}
		if err := addBoxFeature(c, [][]float64{{-0.55, side*0.72 - 0.09}, {0.55, side*0.72 + 0.09}}, "11 mm", "1.8 mm", "1.2 mm", "join"); err != nil {
			return err
		}
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 6, []float64{0, 0}, "2.2 mm", "2.2 mm")
	return c.applyFeature("extrude", map[string]any{"sketchIndex": 6, "profileIndex": 0, "operation": "cut", "extent": "through-all"})
}
