// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runDraft is the live driver for the DRAFT feature (mould taper — what OpenSCAD fakes with
// linear_extrude(scale=…)): a box whose four side faces are tapered inward about +Z, becoming
// a truncated pyramid. Shows a tapered box in the viewport. (A negative angle tapers inward.)
func runDraft(c *caller) error {
	const a, h = 4.0, 2.0
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.ids(map[string]any{"sketchIndex": 0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {a / 2, a / 2}}})
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"}); err != nil {
		return err
	}
	sides := c.facesBetweenZ(0.1, h-0.1)
	if len(sides) != 4 {
		return fmt.Errorf("draft: want 4 side faces, found %d", len(sides))
	}
	if err := c.applyFeature("draft", map[string]any{"faceRefs": sides, "angle": "-10 deg"}); err != nil {
		return err
	}
	tan := math.Tan(10 * math.Pi / 180)
	taperedSide := a - 2*h*tan
	return c.checkVolume("tapered box (draft -10°)", (a*a*a-taperedSide*taperedSide*taperedSide)/(6*tan))
}

// facesBetweenZ returns reference keys of faces whose representative point's Z is in (lo,hi)
// — the side faces of an axis-aligned block (excludes the z≈0 and z≈h caps).
func (c *caller) facesBetweenZ(lo, hi float64) []string {
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &rk)
	var keys []string
	for _, b := range rk.Bodies {
		for _, f := range b.Faces {
			if len(f.Point) == 3 && f.Point[2] > lo && f.Point[2] < hi {
				keys = append(keys, f.Key)
			}
		}
	}
	return keys
}
