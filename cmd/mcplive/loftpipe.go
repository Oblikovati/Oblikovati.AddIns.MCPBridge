// SPDX-License-Identifier: GPL-2.0-only

package main

import "fmt"

// runLoftPipe is the live driver for a tapered hollow pipe: a loft between two ANNULUS sections
// (outer + inner concentric circles) on parallel planes — a nozzle / reducer / duct. It exercises
// the loft's direct tube meshing: the single inner loop per section is skinned into a watertight
// bore rather than cut, so a coplanar end cap can't leave the pipe open. Shows a tapered tube
// (you can see straight through the bore) in the viewport.
func runLoftPipe(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "ro1", "expression": "20 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "ri1", "expression": "15 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "ro2", "expression": "14 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "ri2", "expression": "10 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "h", "expression": "40 mm"}, nil)

	pickHole := func(sketchIndex int, outer, inner string, ro, ri string) (int, error) {
		oc := c.ids(map[string]any{"sketchIndex": sketchIndex, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": outer})
		ic := c.ids(map[string]any{"sketchIndex": sketchIndex, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": inner})
		if c.err != nil || len(oc) < 2 || len(ic) < 2 {
			return -1, fmt.Errorf("annulus circles reply: outer=%v inner=%v (%v)", oc, ic, c.err)
		}
		c.json("add_sketch_constraint", map[string]any{"sketchIndex": sketchIndex, "kind": "ground", "entities": []uint64{oc[1]}}, nil)
		c.json("add_sketch_constraint", map[string]any{"sketchIndex": sketchIndex, "kind": "coincident", "entities": []uint64{oc[1], ic[1]}}, nil)
		c.dimAt(sketchIndex, "radius", ro, oc[0])
		c.dimAt(sketchIndex, "radius", ri, ic[0])
		if err := c.requireConstrainedAt(sketchIndex); err != nil {
			return -1, err
		}
		idx, _ := c.holeProfile(sketchIndex)
		if idx < 0 {
			return -1, fmt.Errorf("sketch %d has no annular (holed) profile", sketchIndex)
		}
		return idx, nil
	}

	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	p0, err := pickHole(0, "2 cm", "1.5 cm", "ro1", "ri1")
	if err != nil {
		return err
	}

	var wp struct {
		Index int `json:"index"`
	}
	c.json("create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "h"}, &wp)
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	c.json("create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	p1, err := pickHole(sk.SketchIndex, "1.4 cm", "1.0 cm", "ro2", "ri2")
	if err != nil {
		return err
	}

	if err := c.applyFeature("loft", map[string]any{"sections": []map[string]any{
		{"sketchIndex": 0, "profileIndex": p0},
		{"sketchIndex": sk.SketchIndex, "profileIndex": p1},
	}}); err != nil {
		return err
	}

	// Tube volume = outer cone frustum − inner cone frustum (cm). cone(R,r,h)=π·h/3·(R²+Rr+r²).
	cone := func(rr, r, hh float64) float64 { return 3.141592653589793 * hh / 3 * (rr*rr + rr*r + r*r) }
	pipe := func(ro1, ri1, ro2, ri2, hMM float64) float64 {
		h := hMM / 10
		return cone(ro1/10, ro2/10, h) - cone(ri1/10, ri2/10, h)
	}
	if err := c.checkVolume("h=40 tapered pipe", pipe(20, 15, 14, 10, 40)); err != nil {
		return err
	}
	// Resize the height: the work plane and top sketch track, the tube re-lofts watertight.
	c.json("set_parameter", map[string]any{"name": "h", "expression": "60 mm"}, nil)
	return c.checkVolume("h=60 (taller pipe)", pipe(20, 15, 14, 10, 60))
}

// holeProfile returns the index and area of a sketch's first closed profile with a hole.
func (c *caller) holeProfile(sketchIndex int) (int, float64) {
	var p struct {
		Profiles []struct {
			Index int     `json:"index"`
			Area  float64 `json:"area"`
			Holes int     `json:"holes"`
		} `json:"profiles"`
	}
	c.json("list_sketch_profiles", map[string]any{"sketchIndex": sketchIndex}, &p)
	for _, pr := range p.Profiles {
		if pr.Holes > 0 {
			return pr.Index, pr.Area
		}
	}
	return -1, 0
}
