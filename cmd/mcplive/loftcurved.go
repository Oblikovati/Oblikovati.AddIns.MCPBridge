// SPDX-License-Identifier: GPL-2.0-only

package main

import "fmt"

// runLoftCurved is the live driver for an end-CONDITION loft (Slice 2): two EQUAL circles lofted
// with a 30° Angle takeoff at both ends, weighted by an impact. A Free loft of equal circles is
// a straight cylinder; the angle takeoff curves the walls into a barrel — visible in the viewport
// and measurable as extra surface area (a ruled cylinder here is ~75 cm²).
func runLoftCurved(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "r", "expression": "20 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "takeoff", "expression": "30 deg"}, nil)

	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c0 := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	if c.err != nil || len(c0) < 2 {
		return fmt.Errorf("bottom circle reply: %v (%v)", c0, c.err)
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
	c1 := c.ids(map[string]any{"sketchIndex": sk.SketchIndex, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	if c.err != nil || len(c1) < 2 {
		return fmt.Errorf("top circle reply: %v (%v)", c1, c.err)
	}
	c.json("add_sketch_constraint", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "ground", "entities": []uint64{c1[1]}}, nil)
	c.dimAt(sk.SketchIndex, "radius", "r", c1[0])
	if err := c.requireConstrainedAt(sk.SketchIndex); err != nil {
		return err
	}

	angled := map[string]any{"condition": "angle", "angle": "takeoff", "impact": 2}
	if err := c.applyFeature("loft", map[string]any{
		"sections": []map[string]any{{"sketchIndex": 0, "profileIndex": 0}, {"sketchIndex": sk.SketchIndex, "profileIndex": 0}},
		"first":    angled,
		"last":     angled,
	}); err != nil {
		return err
	}
	a30, err := c.partAreaAtLeast("30° barrel (curved walls)", 80)
	if err != nil {
		return err
	}
	// Steeper takeoff (more radial) curves more — the angle is parameter-driven through recompute,
	// so the surface area grows further.
	c.json("set_parameter", map[string]any{"name": "takeoff", "expression": "15 deg"}, nil)
	a15, err := c.partAreaAtLeast("15° barrel (more curved)", 80)
	if err != nil {
		return err
	}
	if a15 <= a30 {
		return fmt.Errorf("steeper takeoff did not curve more: 15° area %.3f <= 30° area %.3f", a15, a30)
	}
	return nil
}

// partAreaAtLeast reads the active part's surface area (cm²), asserts it is at least want (the
// curved loft exceeds the ruled cylinder), and returns it for relative comparisons.
func (c *caller) partAreaAtLeast(tag string, want float64) (float64, error) {
	var pp struct {
		Area float64 `json:"area"`
	}
	c.json("get_physical_properties", nil, &pp)
	if c.err != nil {
		return 0, c.err
	}
	fmt.Printf("  %-26s area = %.3f cm^2  want >= %.1f\n", tag, pp.Area, want)
	if pp.Area < want {
		return pp.Area, fmt.Errorf("%s area %.3f below %.1f (walls not curved)", tag, pp.Area, want)
	}
	return pp.Area, nil
}
