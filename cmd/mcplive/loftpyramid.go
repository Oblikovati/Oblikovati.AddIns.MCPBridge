// SPDX-License-Identifier: GPL-2.0-only

package main

// runLoftPyramid is the live driver for a truncated pyramid: a loft between two SQUARE
// sections of different size on parallel planes (a hopper / transition / lamp base). It
// exercises the loft's corner correspondence (the existing loft blends circles); shows a
// tapered square frustum in the viewport.
func runLoftPyramid(c *caller) error {
	// Bottom square side 2 (half 1) on XY.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.ids(map[string]any{"sketchIndex": 0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {1, 1}}})
	// Top square side 1 (half 0.5) on a work plane offset 15 mm above XY.
	var wp struct {
		Index int `json:"index"`
	}
	c.json("create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "15 mm"}, &wp)
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	c.json("create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	c.ids(map[string]any{"sketchIndex": sk.SketchIndex, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {0.5, 0.5}}})
	if c.err != nil {
		return c.err
	}
	if err := c.applyFeature("loft", map[string]any{
		"sections": []map[string]any{{"sketchIndex": 0, "profileIndex": 0}, {"sketchIndex": sk.SketchIndex, "profileIndex": 0}},
	}); err != nil {
		return err
	}
	// Prismatoid h/6·(a²+(a+b)²+b²), a=2,b=1,h=1.5 → 3.5 cm³ (square corners are exact).
	return c.checkVolume("truncated pyramid", 1.5/6*(2*2+(2+1)*(2+1)+1*1))
}
