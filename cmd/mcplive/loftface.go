// SPDX-License-Identifier: GPL-2.0-only

package main

import "fmt"

// runLoftFace is the live driver for a FACE-section loft (Slice 2c): a cylinder whose top face is
// lofted up to a smaller circle with a Tangent condition, so the loft leaves the planar top
// tangent to it — a smooth trumpet flare (exact G1) rather than a straight cone. Joined to the
// cylinder it reads as one flared solid in the viewport.
func runLoftFace(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c0 := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	if c.err != nil || len(c0) < 2 {
		return fmt.Errorf("base circle reply: %v (%v)", c0, c.err)
	}
	c.con("ground", c0[1])
	c.dim("radius", "2 cm", c0[0])
	if err := c.requireConstrainedAt(0); err != nil {
		return err
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "30 mm"}); err != nil {
		return err
	}
	faceRef := c.topFaceKey()
	if faceRef == "" {
		return fmt.Errorf("no top face found for the cylinder")
	}

	var wp struct {
		Index int `json:"index"`
	}
	c.json("create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "60 mm"}, &wp)
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	c.json("create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	c1 := c.ids(map[string]any{"sketchIndex": sk.SketchIndex, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "1 cm"})
	if c.err != nil || len(c1) < 2 {
		return fmt.Errorf("top circle reply: %v (%v)", c1, c.err)
	}
	c.json("add_sketch_constraint", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "ground", "entities": []uint64{c1[1]}}, nil)
	c.dimAt(sk.SketchIndex, "radius", "1 cm", c1[0])
	if err := c.requireConstrainedAt(sk.SketchIndex); err != nil {
		return err
	}

	if err := c.applyFeature("loft", map[string]any{
		"sections":  []map[string]any{{"faceRef": faceRef}, {"sketchIndex": sk.SketchIndex, "profileIndex": 0}},
		"first":     map[string]any{"condition": "tangent"},
		"operation": "new",
	}); err != nil {
		return err
	}
	// Cylinder πr²h ≈ 37.7 cm³ + the tangent flare (a ruled cone would add ~22, the flare adds more):
	// the two-body total clears 60 cm³, which a ruled loft would not.
	return c.checkVolumeAtLeast("tangent face flare (trumpet)", 60)
}

// checkVolumeAtLeast asserts the active part's volume is at least want cm³, printing it for a
// visual cross-check.
func (c *caller) checkVolumeAtLeast(tag string, want float64) error {
	var pp struct {
		Volume float64 `json:"volume"`
	}
	c.json("get_physical_properties", nil, &pp)
	if c.err != nil {
		return c.err
	}
	fmt.Printf("  %-30s volume = %.3f cm^3  want >= %.1f\n", tag, pp.Volume, want)
	if pp.Volume < want {
		return fmt.Errorf("%s volume %.3f below %.1f (loft not flaring)", tag, pp.Volume, want)
	}
	return nil
}
