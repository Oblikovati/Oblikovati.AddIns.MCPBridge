// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"os"
)

// arialPaths are where this machine keeps Arial (true-type emboss test font), falling back
// to Liberation Sans — metric-compatible with Arial, and its 'A' has the same closed
// profile + counter hole the scenario relies on.
var arialPaths = []string{
	"/home/vmiguel/.steam/debian-installation/steamapps/common/Proton - Experimental/files/share/fonts/arial.ttf",
	"/usr/share/fonts/truetype/msttcorefonts/Arial.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
}

// runEmboss is the live driver for true-type emboss: it raises a real letter onto a plate.
// The letter is a sketch TEXT entity (sketch.addText with a font); the emboss references it
// BY ENTITY ID, so the glyph geometry (closed profile with its counter hole) is derived at
// recompute, never baked into the sketch. Shows a 3D 'A' on a plate in the viewport.
func runEmboss(c *caller) error {
	font := ""
	for _, p := range arialPaths {
		if _, err := os.Stat(p); err == nil {
			font = p
			break
		}
	}
	if font == "" {
		return fmt.Errorf("emboss: no Arial/Liberation .ttf found on this machine")
	}

	// Plate 2×2 cm, 5 mm thick, top face at z=0.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.ids(map[string]any{"sketchIndex": 0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {1, 1}}})
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "5 mm", "operation": "new", "direction": "negative"}); err != nil {
		return err
	}
	// 'A' as a text entity on XY, embossed by reference to its entity id.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	var ent struct {
		EntityID uint64 `json:"entityId"`
	}
	c.json("add_sketch_text", map[string]any{"sketchIndex": 1, "anchor": []float64{-0.35, -0.35}, "text": "A", "height": "10 mm", "font": font}, &ent)
	if c.err != nil {
		return c.err
	}
	if ent.EntityID == 0 {
		return fmt.Errorf("emboss: add_sketch_text returned no entity id")
	}
	if err := c.applyFeature("emboss", map[string]any{"sketchIndex": 1, "textEntity": ent.EntityID, "depth": "1.5 mm"}); err != nil {
		return err
	}
	// Volume = plate + glyphArea×depth. The glyph area of a 10 mm 'A' is ~0.15–0.30 cm²,
	// so the raise adds ~0.02–0.05 cm³ on top of the 2.0 cm³ plate.
	const plate = 2.0 * 2.0 * 0.5
	got := c.volume()
	raise := got - plate
	fmt.Printf("  'A' embossed by text ref  volume = %.6f cm^3  (raise %.4f)\n", got, raise)
	if raise < 0.01 || raise > 0.10 {
		return fmt.Errorf("emboss raise = %.4f cm^3, want a glyph-sized raise in [0.01, 0.10]", raise)
	}
	return nil
}
