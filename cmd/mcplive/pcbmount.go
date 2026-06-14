// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runPcbMount is the live driver for NopSCADlib's pcb_mount — the corpus's JOIN-pattern part: a
// base plate plus a 2×2 rectangular pattern of standoff posts, where every patterned occurrence
// ADDS a post (the replicate path that re-applies a Join tool, unlike the hole/slot cut
// patterns). Proves the join pattern over the live C-ABI stack: base + four posts.
func runPcbMount(c *caller) error {
	for _, p := range [][2]string{{"L", "40 mm"}, {"W", "30 mm"}, {"bt", "3 mm"}, {"pr", "3 mm"}, {"postLen", "11 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	// Base plate: L×W rectangle (corner at origin) extruded bt.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	r := c.ids(map[string]any{"sketchIndex": 0, "kind": "rectangle", "points": [][]float64{{0, 0}, {4, 3}}})
	if len(r) < 5 {
		return fmt.Errorf("rectangle reply too short: %v", r)
	}
	bl, br, tr, tl := r[1], r[2], r[3], r[4]
	c.con("ground", bl)
	c.con("horizontal", bl, br)
	c.con("vertical", bl, tl)
	c.con("horizontal", tl, tr)
	c.con("vertical", br, tr)
	c.dim("distance", "L", bl, br)
	c.dim("distance", "W", bl, tl)
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": c.closedProfile(), "distance": "bt", "operation": "new"}); err != nil {
		return err
	}

	// Seed post at a corner, JOINED, standing postLen tall (pokes through the plate, rises above).
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	post := c.ids(map[string]any{"sketchIndex": 1, "kind": "circle", "points": [][]float64{{0.5, 0.5}}, "radius": "0.3 cm"})
	c.conAt(1, "ground", post[1])
	c.dimAt(1, "diameter", "2*pr", post[0])
	if err := c.requireConstrainedAt(1); err != nil {
		return err
	}
	postName, err := c.applyNamed("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "operation": "join", "distance": "postLen"})
	if err != nil {
		return err
	}

	// THE JOIN PATTERN: 2×2 grid of the post → four corner standoffs.
	if err := c.applyFeature("patternRectangular", map[string]any{
		"sourceFeatures": []string{postName}, "countX": 2, "countY": 2,
		"stepX": []float64{3, 0, 0}, "stepY": []float64{0, 2, 0},
	}); err != nil {
		return err
	}

	if err := c.checkVolume("postLen=11", pcbMountVol(11)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "postLen", "expression": "14 mm"}, nil)
	return c.checkVolume("postLen=14 (taller)", pcbMountVol(14))
}

// pcbMountVol = base plate + four posts' material above the plate, cm^3 (postLen in mm).
func pcbMountVol(postLenMM float64) float64 {
	const L, W, bt, rr = 4.0, 3.0, 0.3, 0.3
	return L*W*bt + 4*math.Pi*rr*rr*(postLenMM/10-bt)
}
