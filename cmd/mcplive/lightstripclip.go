// SPDX-License-Identifier: GPL-2.0-only

package main

func runLightStripClip(c *caller) error {
	for _, p := range [][2]string{{"wall", "1.8 mm"}, {"slotW", "10.2 mm"}, {"apertureW", "6 mm"}, {"clipDepth", "10 mm"}, {"clipSide", "3 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	outerArea := lightStripClipArea()
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedRect(c, 0, [][]float64{{-0.69, -0.18}, {0.69, 0.48}}, "slotW + 2 * wall", "clipSide + 2 * wall")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "clipDepth", "operation": "new"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedRect(c, 1, [][]float64{{-0.51, 0}, {0.51, 0.3}}, "slotW", "clipSide")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "extent": "through-all", "operation": "cut"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedRect(c, 2, [][]float64{{-0.3, 0}, {0.3, 0.48}}, "apertureW", "clipSide + wall")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 2, "profileIndex": 0, "extent": "through-all", "operation": "cut"}); err != nil {
		return err
	}
	if err := c.checkVolumeTol("depth=10", outerArea*1.0, 0.02); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "clipDepth", "expression": "16 mm"}, nil)
	return c.checkVolumeTol("depth=16", outerArea*1.6, 0.02)
}
