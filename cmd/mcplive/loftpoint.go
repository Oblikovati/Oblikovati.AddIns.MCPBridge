// SPDX-License-Identifier: GPL-2.0-only

package main

import "fmt"

// runLoftPoint is the live driver for a POINT-section loft (Slice 2b): a circle lofted to an apex
// (a point section). With a Sharp condition it is a straight cone (V = πr²h/3); with TangentToPlane
// it domes to a rounded tip. Shows a cone/dome in the viewport.
func runLoftPoint(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "r", "expression": "20 mm"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c0 := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	if c.err != nil || len(c0) < 2 {
		return fmt.Errorf("base circle reply: %v (%v)", c0, c.err)
	}
	c.con("ground", c0[1])
	c.dim("radius", "r", c0[0])
	if err := c.requireConstrainedAt(0); err != nil {
		return err
	}

	var wp struct {
		Index int `json:"index"`
	}
	c.json("create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "40 mm"}, &wp)
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	c.json("create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	if c.err != nil {
		return c.err
	}

	// Sharp apex → a cone.
	if err := c.applyFeature("loft", map[string]any{
		"sections": []map[string]any{
			{"sketchIndex": 0, "profileIndex": 0},
			{"sketchIndex": sk.SketchIndex, "point": []float64{0, 0}},
		},
		"last": map[string]any{"condition": "sharp"},
	}); err != nil {
		return err
	}
	return c.checkVolume("cone (sharp apex)", 3.141592653589793*2*2/3*4)
}
