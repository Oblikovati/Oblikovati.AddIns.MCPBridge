// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runFixingBlock is the live driver for NopSCADlib's fixing_block — the corpus's mirror-feature
// part. A block with two vertical screw bores that are a MIRROR PAIR about the centre plane,
// plus one centred horizontal bore. The left bore is a sketched cut-extrude; the right bore is
// that cut MIRRORED across the YZ plane (x = W/2). Proves the model mirror feature reproduces a
// cut over the live C-ABI stack (it once added material — geom.TransformSurface flipped a
// reflected plane's normal inward; see TestTransformedReflectedToolCuts).
func runFixingBlock(c *caller) error {
	for _, p := range [][2]string{{"W", "24 mm"}, {"D", "12 mm"}, {"th", "12 mm"}, {"vd", "4 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	// Block: a W×D rectangle (corner at origin) extruded th; centre plane is x = W/2.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	r := c.ids(map[string]any{"sketchIndex": 0, "kind": "rectangle", "points": [][]float64{{0, 0}, {2.4, 1.2}}})
	if len(r) < 5 {
		return fmt.Errorf("rectangle reply too short: %v", r)
	}
	// ids() prepends the entity id; rectangle returns its 4 corner point ids after it.
	bl, br, tr, tl := r[1], r[2], r[3], r[4]
	c.con("ground", bl)
	c.con("horizontal", bl, br)
	c.con("vertical", bl, tl)
	c.con("horizontal", tl, tr)
	c.con("vertical", br, tr)
	c.dim("distance", "W", bl, br)
	c.dim("distance", "D", bl, tl)
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": c.closedProfile(), "distance": "th", "operation": "new"}); err != nil {
		return err
	}

	// Seed vertical bore at x = 6 mm, cut through-all; centre grounded + diameter dim ⇒ 0 DOF.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	seed := c.ids(map[string]any{"sketchIndex": 1, "kind": "circle", "points": [][]float64{{0.6, 0.6}}, "radius": "0.2 cm"})
	c.conAt(1, "ground", seed[1])
	c.dimAt(1, "diameter", "vd", seed[0])
	if err := c.requireConstrainedAt(1); err != nil {
		return err
	}
	bore, err := c.applyNamed("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "operation": "cut", "extent": "through-all"})
	if err != nil {
		return err
	}

	// THE MIRROR: reflect the bore cut across the YZ centre plane (x = 1.2 cm) → second bore.
	if err := c.applyFeature("mirror", map[string]any{"sourceFeatures": []string{bore}, "origin": []float64{1.2, 0, 0}, "normal": []float64{1, 0, 0}}); err != nil {
		return err
	}
	// Centred horizontal cross bore (single, on the mirror plane).
	if err := c.applyFeature("hole", map[string]any{"faceRef": c.frontFaceKey(), "diameter": "vd"}); err != nil {
		return err
	}

	if err := c.checkVolume("th=12", fixingBlockVol(12)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "th", "expression": "16 mm"}, nil)
	return c.checkVolume("th=16 (thicker)", fixingBlockVol(16))
}

// fixingBlockVol = block − two vertical through bores (height th) − one horizontal bore (D), cm^3.
func fixingBlockVol(thMM float64) float64 {
	const W, D, rr = 2.4, 1.2, 0.2
	th := thMM / 10
	return W*D*th - 2*math.Pi*rr*rr*th - math.Pi*rr*rr*D
}

// applyNamed adds a feature and returns its assigned name (for mirror/pattern sourceFeatures).
func (c *caller) applyNamed(kind string, args map[string]any) (string, error) {
	var r struct {
		Feature string `json:"feature"`
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	c.json("add_feature", map[string]any{"kind": kind, "args": args}, &r)
	if c.err != nil {
		return "", c.err
	}
	if !r.Healthy {
		return "", fmt.Errorf("%s unhealthy: %s", kind, r.Reason)
	}
	return r.Feature, nil
}

// frontFaceKey returns the reference key of the front (smallest-Y) planar face.
func (c *caller) frontFaceKey() string {
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &rk)
	best, bestY := "", math.Inf(1)
	if len(rk.Bodies) > 0 {
		for _, f := range rk.Bodies[0].Faces {
			if len(f.Point) == 3 && f.Point[1] < bestY {
				best, bestY = f.Key, f.Point[1]
			}
		}
	}
	return best
}
