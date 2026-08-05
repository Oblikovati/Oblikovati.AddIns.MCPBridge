// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// runSmoothWire is the live check for Oblikovati#1643 (audit S8): the smooth (G2)
// constraint — enumerable and persistable since M21, but never wire-creatable — must now
// be accepted by sketch.addConstraint with the same nearest-endpoint join the app tool
// makes, and must still refuse a spline-less pair (two analytic curves can only be G2
// when cocircular).
func runSmoothWire(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	line := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0}, {1, 0}}})
	spline := c.ids(map[string]any{"sketchIndex": 0, "kind": "spline", "points": [][]float64{{1, 0}, {2, 1}, {3, 0}}})
	if c.err != nil {
		return c.err
	}
	var r struct {
		Kind string `json:"kind"`
		DOF  int    `json:"dof"`
	}
	c.json("add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "smooth", "entities": []uint64{line[0], spline[0]}}, &r)
	if c.err != nil {
		return c.err
	}
	if r.Kind != "smooth" {
		return fmt.Errorf("add_sketch_constraint returned kind %q, want smooth", r.Kind)
	}
	fmt.Printf("  smooth accepted over the wire, sketch DOF now %d\n", r.DOF)
	return requireSmoothRefusesSplineless(c)
}

// requireSmoothRefusesSplineless asserts the guard rail: smooth over two lines (no
// adjustable-curvature side) must be an error reply, not a silent constraint.
func requireSmoothRefusesSplineless(c *caller) error {
	l1 := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{5, 0}, {6, 0}}})
	l2 := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{6, 0}, {7, 1}}})
	if c.err != nil {
		return c.err
	}
	res, err := c.cs.CallTool(c.ctx, &mcp.CallToolParams{Name: "add_sketch_constraint", Arguments: map[string]any{
		"sketchIndex": 0, "kind": "smooth", "entities": []uint64{l1[0], l2[0]},
	}})
	if err != nil {
		return fmt.Errorf("splineless smooth probe transport error: %w", err)
	}
	if !res.IsError {
		return fmt.Errorf("smooth over two lines was accepted — the needs-a-spline rule is gone")
	}
	fmt.Printf("  splineless smooth refused as expected: %s\n", firstText(res))
	return nil
}
