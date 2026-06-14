// SPDX-License-Identifier: GPL-2.0-only

package main

func runSquareButton(c *caller) error {
	for _, p := range [][2]string{{"w", "12 mm"}, {"h", "3.5 mm"}, {"rivet", "1.6 mm"}, {"stem", "4 mm"}, {"cap", "6 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	if err := addBoxFeature(c, [][]float64{{-0.6, -0.6}, {0.6, 0.6}}, "w", "w", "h", "new"); err != nil {
		return err
	}
	idx := 1
	for _, x := range []float64{-0.4, 0.4} {
		for _, y := range []float64{-0.4, 0.4} {
			c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
			addConstrainedCircle(c, idx, []float64{x, y}, "0.8 mm", "rivet/2")
			if err := c.applyFeature("extrude", map[string]any{"sketchIndex": idx, "profileIndex": 0, "distance": "4 mm", "operation": "join"}); err != nil {
				return err
			}
			idx++
		}
	}
	for _, cap := range []struct{ d, h string }{{"stem", "3 mm"}, {"cap", "3 mm"}} {
		c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
		addConstrainedCircle(c, idx, []float64{0, 0}, "3 mm", cap.d+"/2")
		if err := c.applyFeature("extrude", map[string]any{"sketchIndex": idx, "profileIndex": 0, "distance": cap.h, "operation": "join"}); err != nil {
			return err
		}
		idx++
	}
	if got := c.volume(); got <= 1.2*1.2*0.35 {
		return errVolume("squarebutton", got, 1.2*1.2*0.35)
	}
	return nil
}
