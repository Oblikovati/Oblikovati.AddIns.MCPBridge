// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runHDMI is the live driver for the NopSCADlib HDMI socket metal shell (vitamins/pcb.scad).
// The D() cross-section — the hull of two stacked rectangles — is a keystone hexagon, drawn as a
// closed polyline. offset_sketch grows it by the 0.5 mm wall thickness; the annular wall band is
// extruded the full 12 mm depth into a hollow tube, then a second keystone sketch is JOIN-
// extruded 1 mm to plug one end (the metal flange). Two extrudes whose geometry collides at the
// shared inner walls. After the volume check it frames + normal-debug captures the viewport so
// the shell can be eyeballed (front green / back red).
func runHDMI(c *caller) error {
	keystone := [][]float64{{-0.7, 0.6}, {-0.7, 0.3}, {-0.5, 0.15}, {0.5, 0.15}, {0.7, 0.3}, {0.7, 0.6}}

	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "polyline", "closed": true, "points": keystone}, nil)

	var off struct {
		Created []uint64 `json:"created"`
	}
	c.json("offset_sketch", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "0.5 mm", "arcSegments": 16}, &off)
	if c.err != nil {
		return c.err
	}
	band := c.profileWithHole()
	if band < 0 {
		return fmt.Errorf("no annular wall-band profile after offset")
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": band, "distance": "12 mm", "operation": "new"}); err != nil {
		return err
	}

	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 1, "kind": "polyline", "closed": true, "points": keystone}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "distance": "1 mm", "operation": "join"}); err != nil {
		return err
	}

	bandArea := 3.5*0.05 + math.Pi*0.05*0.05 // perim·t + π·t² (cm²)
	if err := c.checkVolume("hdmi shell", bandArea*1.2+0.6*0.1); err != nil {
		return err
	}

	// Frame + normal-debug capture for visual inspection.
	c.json("execute_command", map[string]any{"id": "View.Home"}, nil)
	c.json("set_normal_debug", map[string]any{"on": true}, nil)
	c.json("capture_viewport", map[string]any{"path": "/tmp/oblikovati-capture.png"}, nil)
	return c.err
}
