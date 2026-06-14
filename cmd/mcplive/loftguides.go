// SPDX-License-Identifier: GPL-2.0-only

package main

import "fmt"

// runLoftAreaGraph is the live driver for an AREA-GRAPH loft (Slice 5, kLoftWithAreaGraphSections):
// two equal circles lofted with an area graph that doubles the mid cross-section area — a barrel
// controlled by area rather than a rail. Shows the bulged middle in the viewport.
func runLoftAreaGraph(c *caller) error {
	top, err := c.twoEqualCircles()
	if err != nil {
		return err
	}
	if err := c.applyFeature("loft", map[string]any{
		"sections":  []map[string]any{{"sketchIndex": 0, "profileIndex": 0}, {"sketchIndex": top, "profileIndex": 0}},
		"areaGraph": []map[string]any{{"t": 0.5, "scale": 2.0}},
	}); err != nil {
		return err
	}
	return c.checkVolumeAtLeast("area-graph barrel (2× mid area)", 55)
}

// runLoftMapCurve is the live driver for a MAP-CURVE loft (Slice 6, MapPointCurves): two equal
// squares with an explicit correspondence pairing a corner with the next corner — a 90° twist the
// auto-alignment would never choose. The twist pinches the mid section (less volume than the prism).
func runLoftMapCurve(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.ids(map[string]any{"sketchIndex": 0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {2, 2}}})
	var wp struct {
		Index int `json:"index"`
	}
	c.json("create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "40 mm"}, &wp)
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	c.json("create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	c.ids(map[string]any{"sketchIndex": sk.SketchIndex, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {2, 2}}})
	if c.err != nil {
		return c.err
	}
	if err := c.applyFeature("loft", map[string]any{
		"sections":  []map[string]any{{"sketchIndex": 0, "profileIndex": 0}, {"sketchIndex": sk.SketchIndex, "profileIndex": 0}},
		"mapCurves": []map[string]any{{"points": [][]float64{{2, 2, 0}, {-2, 2, 4}}}},
	}); err != nil {
		return err
	}
	return c.checkVolumeAtMost("map-curve twist (pinched mid)", 58) // plain prism is 64 cm³
}

// twoEqualCircles builds two r=2 circles 40 mm apart (XY + a work plane), returning the top sketch.
func (c *caller) twoEqualCircles() (int, error) {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c0 := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	if c.err != nil || len(c0) < 2 {
		return 0, fmt.Errorf("bottom circle reply: %v (%v)", c0, c.err)
	}
	c.con("ground", c0[1])
	c.dim("radius", "2 cm", c0[0])
	if err := c.requireConstrainedAt(0); err != nil {
		return 0, err
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
		return 0, fmt.Errorf("top circle reply: %v (%v)", c1, c.err)
	}
	c.json("add_sketch_constraint", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "ground", "entities": []uint64{c1[1]}}, nil)
	c.dimAt(sk.SketchIndex, "radius", "2 cm", c1[0])
	if err := c.requireConstrainedAt(sk.SketchIndex); err != nil {
		return 0, err
	}
	return sk.SketchIndex, nil
}

// checkVolumeAtMost asserts the active part's volume is at most want cm³ (e.g. a twist that pinches
// the body below the un-twisted prism), printing it for a visual cross-check.
func (c *caller) checkVolumeAtMost(tag string, want float64) error {
	var pp struct {
		Volume float64 `json:"volume"`
	}
	c.json("get_physical_properties", nil, &pp)
	if c.err != nil {
		return c.err
	}
	fmt.Printf("  %-30s volume = %.3f cm^3  want <= %.1f\n", tag, pp.Volume, want)
	if pp.Volume > want {
		return fmt.Errorf("%s volume %.3f above %.1f (correspondence unchanged)", tag, pp.Volume, want)
	}
	return nil
}
