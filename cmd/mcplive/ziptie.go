// SPDX-License-Identifier: GPL-2.0-only

package main

func runZiptie(c *caller) error {
	for _, p := range [][2]string{{"tieW", "3.6 mm"}, {"strapT", "1.8 mm"}, {"latchW", "3.5 mm"}, {"latchH", "3.2 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	outer := stadiumBandPoints2D(0, 0, 1.0, 0.45, 24)
	inner := stadiumBandPoints2D(0, 0, 0.82, 0.27, 24)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "polyline", "closed": true, "points": outer}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "tieW", "operation": "new"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 1, "kind": "polyline", "closed": true, "points": inner}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "operation": "cut", "extent": "through-all"}); err != nil {
		return err
	}
	strapVolume := c.volume()
	if err := addBoxFeature(c, [][]float64{{0.65, -0.16}, {1.0, 0.16}}, "latchW", "latchH", "tieW", "join"); err != nil {
		return err
	}
	if got := c.volume(); got <= strapVolume {
		return errVolume("ziptie", got, strapVolume)
	}
	return nil
}
