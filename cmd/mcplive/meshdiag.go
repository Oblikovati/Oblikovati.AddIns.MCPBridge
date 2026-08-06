// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// runMeshDiag is the live check for Oblikovati#2058: the tessellator's report now rides with the
// feature that produced the body, instead of dying inside kernel/ops. Two halves, because a "no
// tessellate.* codes" assertion on its own would also pass if the whole channel were dead:
//
//   - a POSITIVE control first — a cylinder-on-cylinder cut whose reply must carry
//     boolean.analytic-faceted (the #1601 channel), proving the diagnostics array in THIS session is
//     wired and reaches the caller non-empty;
//   - then the false-positive gate — a clean exact model (the #2038 seam bore, plus a fillet and a
//     hole) whose replies must carry NO tessellate.* code, at 1x and at 20x.
//
// There is deliberately no live model that DOES raise a tessellation defect: the only geometry that
// still degrades is malformed input (a self-crossing trim), and a body the kernel builds and then
// meshes badly is a bug to fix, not a fixture to drive. That case is covered by the kernel and wire
// regressions against test-utilities/degenerate.
func runMeshDiag(c *caller) error {
	if err := meshDiagPositiveControl(c); err != nil {
		return err
	}
	return meshDiagCleanModel(c)
}

// meshDiagPositiveControl proves the reply's diagnostics array is live in this session.
func meshDiagPositiveControl(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "20 mm", "20 mm")
	if err := c.applyFeature("extrude", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "distance": "30 mm", "operation": "new",
	}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 1, []float64{1.5, 0}, "10 mm", "10 mm")
	diags, err := c.featureDiagnostics("extrude", map[string]any{
		"sketchIndex": 1, "profileIndex": 0, "distance": "30 mm", "operation": "cut",
	})
	if err != nil {
		return err
	}
	if !hasCode(diags, "boolean.analytic-faceted") {
		return fmt.Errorf("positive control: the crossing-cylinder cut reply carries no "+
			"boolean.analytic-faceted diagnostic, so a later empty array would prove nothing: %v", diags)
	}
	fmt.Printf("  positive control: diagnostics array reaches the caller — %v\n", diags)
	return nil
}

// meshDiagCleanModel rebuilds the #2038 seam bore (the case whose 77%-low body was reported with an
// empty diagnostics array) and asserts the report is quiet AND the volume right, at both scales.
func meshDiagCleanModel(c *caller) error {
	c.json("close_all_documents", map[string]any{"force": true}, nil)
	c.json("create_document", map[string]any{"type": "part", "name": "meshdiag-clean"}, nil)
	for _, p := range [][2]string{{"diskR", "50 mm"}, {"diskH", "4 mm"}, {"rodR", "1.5 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "50 mm", "diskR")
	if err := c.checkQuiet("disk", "extrude", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "distance": "diskH", "operation": "new", "direction": "symmetric",
	}); err != nil {
		return err
	}
	// YZ ⇒ the tunnel runs along X, through the disk's +X seam: #2038's geometry exactly.
	c.json("create_sketch", map[string]any{"plane": "YZ"}, nil)
	addConstrainedCircle(c, 1, []float64{0, 0}, "1.5 mm", "rodR")
	if err := c.checkQuiet("seam bore", "extrude", map[string]any{
		"sketchIndex": 1, "profileIndex": 0, "operation": "cut",
		"extent": "through-all", "direction": "symmetric",
	}); err != nil {
		return err
	}
	if err := c.checkVolumeTol("meshdiag", math.Pi*5*5*0.4-math.Pi*0.15*0.15*10, 0.01); err != nil {
		return err
	}
	return meshDiagAtScale(c, 20)
}

// meshDiagAtScale re-drives the model k times larger and times the rebuild, so the extra display-
// tolerance mesh the report costs shows up as a number rather than as a feeling.
func meshDiagAtScale(c *caller, k float64) error {
	for _, p := range [][2]string{{"diskR", "1000 mm"}, {"diskH", "80 mm"}, {"rodR", "30 mm"}} {
		c.json("set_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	start := time.Now()
	if err := c.checkVolumeTol(fmt.Sprintf("meshdiag %gx", k),
		math.Pi*(5*k)*(5*k)*(0.4*k)-math.Pi*(0.15*k)*(0.15*k)*(10*k), 0.01); err != nil {
		return err
	}
	fmt.Printf("  %gx rebuild + measure took %v\n", k, time.Since(start))
	return nil
}

// checkQuiet applies a feature and fails if its reply carries any tessellation diagnostic — the
// false-positive gate. A channel that cries on healthy geometry is one users learn to ignore.
func (c *caller) checkQuiet(tag, kind string, args map[string]any) error {
	diags, err := c.featureDiagnostics(kind, args)
	if err != nil {
		return err
	}
	for _, d := range diags {
		if strings.HasPrefix(d.Code, "tessellate.") {
			return fmt.Errorf("%s: healthy exact geometry reported %s — %s", tag, d.Code, d.Detail)
		}
	}
	fmt.Printf("  %s: quiet (%d diagnostics)\n", tag, len(diags))
	return nil
}
