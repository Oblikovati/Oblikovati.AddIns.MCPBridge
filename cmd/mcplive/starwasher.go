// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runStarWasher models a slotted (star) washer: an annular disc with 12 radial slots cut by a
// CIRCULAR pattern of a rectangle, extruded (a 13-hole cap → earcut). Widening the disc tracks
// the slot bolt-circle outward.
func runStarWasher(c *caller) error {
	for _, p := range [][2]string{
		{"starD", "18 mm"}, {"bore", "6 mm"}, {"slotLen", "4 mm"}, {"slotW", "1.5 mm"}, {"th", "1 mm"},
	} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)

	o := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})
	outer := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.9 cm"})
	boreC := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.3 cm"})
	var rect struct {
		EntityIDs []uint64 `json:"entityIds"`
		PointIDs  []uint64 `json:"pointIds"`
	}
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "rectangle", "points": [][]float64{{0.4, -0.075}, {0.8, 0.075}}}, &rect)
	m := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0.4, 0}}})
	if c.err != nil || len(o) < 1 || len(outer) < 2 || len(boreC) < 2 || len(rect.EntityIDs) < 4 || len(rect.PointIDs) < 4 || len(m) < 1 {
		return fmt.Errorf("star-washer entity replies too short (%v)", c.err)
	}
	bl, br, tr, tl := rect.PointIDs[0], rect.PointIDs[1], rect.PointIDs[2], rect.PointIDs[3]

	c.con("ground", o[0])
	c.con("coincident", outer[1], o[0])
	c.dim("radius", "starD/2", outer[0])
	c.con("coincident", boreC[1], o[0])
	c.dim("radius", "bore/2", boreC[0])
	c.con("horizontal", bl, br)
	c.con("vertical", bl, tl)
	c.con("horizontal", tl, tr)
	c.con("vertical", br, tr)
	c.con("horizontal", o[0], m[0])
	c.dim("distance", "(bore/2 + starD/2)/2 - slotLen/2", o[0], m[0])
	c.con("midpoint", m[0], rect.EntityIDs[3]) // left edge
	c.dim("distance", "slotLen", bl, br)
	c.dim("distance", "slotW", bl, tl)

	c.json("add_sketch_pattern", map[string]any{
		"sketchIndex": 0, "kind": "circular", "entities": rect.EntityIDs,
		"count": 12, "angle": "360 deg", "center": []float64{0, 0},
	}, nil)
	if err := c.requireConstrained(); err != nil {
		return err
	}

	prof := c.profileWithHole()
	if prof < 0 {
		return fmt.Errorf("no annular slotted profile")
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": prof, "distance": "th", "operation": "new"}); err != nil {
		return err
	}
	vol := func(starDmm float64) float64 {
		R, rb, sl, sw, tt := starDmm/20, 0.3, 0.4, 0.15, 0.1
		return (math.Pi*R*R - math.Pi*rb*rb - 12*sl*sw) * tt
	}
	if err := c.checkVolumeTol("starD=18", vol(18), 0.03); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "starD", "expression": "22 mm"}, nil)
	return c.checkVolumeTol("starD=22 (wider)", vol(22), 0.03)
}
