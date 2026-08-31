// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runMixedBoolean is the live driver for the ADR-0058 mixed per-face boolean (#2247): a plate with a
// cylindrical boss THROUGH its top face (a mixed body — the top face carries the boss rim circle as a
// curved hole loop), then a rectangular pocket cut through the plate away from the boss. The cut runs
// the per-face dispatch end to end over the real C-ABI stack: the seat splits with its hole DETACHED
// and re-attached exactly, the boss wall and cap pass through analytically, and the unified radial
// stitch welds the result. A parameter edit then re-runs the whole recompute. Leaves a screenshot at
// /tmp/oblikovati-mixedboolean.png.
func runMixedBoolean(c *caller) error {
	for _, p := range [][2]string{{"L", "80 mm"}, {"t", "20 mm"},
		{"bossR", "10 mm"}, {"bossH", "35 mm"}, {"pocketW", "20 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	// Plate: L×L square extruded t.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	r := c.ids(map[string]any{"sketchIndex": 0, "kind": "rectangle", "points": [][]float64{{0, 0}, {8, 8}}})
	if len(r) < 5 {
		return fmt.Errorf("plate rectangle reply too short: %v", r)
	}
	bl, br, tr, tl := r[1], r[2], r[3], r[4]
	c.con("ground", bl)
	c.con("horizontal", bl, br)
	c.con("vertical", bl, tl)
	c.con("horizontal", tl, tr)
	c.con("vertical", br, tr)
	c.dim("distance", "L", bl, br)
	c.dim("distance", "L", bl, tl)
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": c.closedProfile(), "distance": "t", "operation": "new"}); err != nil {
		return err
	}
	// Boss: a circle at (6,6) extruded bossH as a JOIN — the cylinder crosses the plate's top face,
	// leaving the top face with the rim circle as a curved hole loop (the mixed-body fixture).
	if err := bossJoin(c); err != nil {
		return err
	}
	if err := c.checkVolume("bossed plate", mixedBooleanVol(2)); err != nil {
		return err
	}
	// Pocket: a through cut far from the boss — the mixed per-face dispatch path under test.
	if err := pocketCut(c); err != nil {
		return err
	}
	if err := c.checkVolume("pocket cut (mixed dispatch)", mixedBooleanVol(2)-2*2*2); err != nil {
		return err
	}
	// Recompute stress: widen the pocket and re-verify the whole chain.
	c.json("set_parameter", map[string]any{"name": "pocketW", "expression": "30 mm"}, nil)
	if err := c.checkVolume("pocketW=30 recompute", mixedBooleanVol(2)-3*3*2); err != nil {
		return err
	}
	c.json("capture_viewport", map[string]any{"path": "/tmp/oblikovati-mixedboolean.png"}, nil)
	return nil
}

// bossJoin sketches the boss circle and joins its extrusion through the plate top.
func bossJoin(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	cc := c.ids(map[string]any{"sketchIndex": 1, "kind": "circle", "points": [][]float64{{6, 6}}, "radius": "bossR"})
	if len(cc) < 1 {
		return fmt.Errorf("boss circle reply too short: %v", cc)
	}
	return c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": c.closedProfile(), "distance": "bossH", "operation": "join"})
}

// pocketCut sketches the pocket square at (1..1+w, 1..1+w) and cuts it through the plate.
func pocketCut(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	p := c.ids(map[string]any{"sketchIndex": 2, "kind": "rectangle", "points": [][]float64{{1, 1}, {3, 3}}})
	if len(p) < 5 {
		return fmt.Errorf("pocket rectangle reply too short: %v", p)
	}
	pbl, pbr, ptr, ptl := p[1], p[2], p[3], p[4]
	conAt2 := func(kind string, ents ...uint64) {
		c.json("add_sketch_constraint", map[string]any{"sketchIndex": 2, "kind": kind, "entities": ents}, nil)
	}
	conAt2("ground", pbl)
	conAt2("horizontal", pbl, pbr)
	conAt2("vertical", pbl, ptl)
	conAt2("horizontal", ptl, ptr)
	conAt2("vertical", pbr, ptr)
	c.dimAt(2, "distance", "pocketW", pbl, pbr)
	c.dimAt(2, "distance", "pocketW", pbl, ptl)
	return c.applyFeature("extrude", map[string]any{"sketchIndex": 2, "profileIndex": c.closedProfile(), "distance": "t", "operation": "cut"})
}

// mixedBooleanVol is the bossed plate's volume in cm^3: L²·t + π·bossR²·(bossH − t).
func mixedBooleanVol(_ float64) float64 {
	return 8*8*2 + math.Pi*1*1*(3.5-2)
}
