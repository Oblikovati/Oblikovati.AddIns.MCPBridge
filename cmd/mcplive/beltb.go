// SPDX-License-Identifier: GPL-2.0-only

package main

func runBeltb(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "8 mm", "8 mm")
	addConstrainedCircle(c, 0, []float64{0, 0}, "6.8 mm", "6.8 mm")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "6 mm", "operation": "new"}); err != nil {
		return err
	}
	if err := addBoxFeature(c, [][]float64{{-0.8, -0.06}, {1.0, 0.06}}, "18 mm", "1.2 mm", "6 mm", "join"); err != nil {
		return err
	}
	if got := c.volume(); got <= 0.12*0.6*1.8 {
		return errVolume("beltb", got, 0.12*0.6*1.8)
	}
	return nil
}
