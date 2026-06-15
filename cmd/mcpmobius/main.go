// SPDX-License-Identifier: GPL-2.0-only

// Command mcpmobius builds a Möbius strip part over a running bridge — a single closed loft
// through N cross-section rectangles arranged around a circle, each tilted by a progressive
// half-twist. It exercises fixed-frame work planes (an explicit origin + in-plane axes per
// section), one sketch + centered rectangle per plane, and a CLOSED loft that wraps the last
// section back to the first; the accumulated 180° twist over the closed loop yields the
// Möbius topology. Watch the viewport as it builds, then it captures a PNG.
//
// The cross-section frame per section follows the three-point/fixed-plane convention
// (xAxis = width direction, yAxis = thickness direction); since the rectangle is anchored
// from the sketch origin, the plane origin is shifted to the band corner so the W×T section
// stays centered on the loop circle of radius R.
//
// Usage: mcpmobius [--url http://127.0.0.1:7800/mcp] [--n 36] [--R 30] [--w 16] [--t 2] [--out /tmp/mobius.png]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint URL")
	n := flag.Int("n", 36, "number of cross-section profiles around the loop")
	R := flag.Float64("R", 30, "loop radius (mm)")
	w := flag.Float64("w", 16, "band width — long cross-section dimension (mm)")
	t := flag.Float64("t", 2, "band thickness — short cross-section dimension (mm)")
	out := flag.String("out", "/tmp/oblikovati-mobius.png", "captured viewport PNG path")
	turns := flag.Float64("turns", 0.5, "twist over a full loop (0.5 = Möbius half-twist; 0 = plain orientable ring; 1 = full twist)")
	bar := flag.Bool("bar", false, "diagnostic: instead of the strip, extrude one w×t rectangle by 2πR (a straight prism) and report its volume — isolates loft vs rectangle/volume-tool errors")
	closed := flag.Bool("closed", true, "close the loft (wrap last section back to first); false leaves a C-shaped open band")
	profile := flag.String("profile", "rect", "cross-section profile: rect | ellipse (both w×t — the elliptical band is rounded)")
	flag.Parse()
	closedLoft = *closed
	if *profile != "rect" && *profile != "ellipse" {
		fmt.Fprintf(os.Stderr, "mcpmobius: --profile must be rect or ellipse, got %q\n", *profile)
		os.Exit(2)
	}
	if *bar {
		if err := barCheck(*url, *R, *w, *t); err != nil {
			fmt.Fprintln(os.Stderr, "mcpmobius:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*url, *n, *R, *w, *t, *out, *turns, *profile); err != nil {
		fmt.Fprintln(os.Stderr, "mcpmobius:", err)
		os.Exit(1)
	}
}

type builder struct {
	ctx context.Context
	cs  *mcp.ClientSession
}

// closedLoft toggles the loft's Closed flag (set from --closed) — a diagnostic knob to
// compare the closed-loop band against an open C-shaped one.
var closedLoft = true

func run(url string, n int, R, w, t float64, out string, turns float64, profile string) error {
	if n < 3 {
		return fmt.Errorf("n must be >= 3, got %d", n)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	mc := mcp.NewClient(&mcp.Implementation{Name: "mcpmobius", Version: "0.1.0"}, nil)
	cs, err := mc.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer func() {
		if closeErr := cs.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "mcpmobius: close session:", closeErr)
		}
	}()
	b := &builder{ctx, cs}

	fmt.Printf("building a Möbius strip over MCP (N=%d, R=%g, w=%g, t=%g, profile=%s) — watch the viewport:\n", n, R, w, t, profile)
	b.call("close_all_documents", map[string]any{"force": true}) // fresh session; see fresh-test-session-close-docs
	b.call("create_document", map[string]any{"type": "part", "name": fmt.Sprintf("mobius-%d", time.Now().UnixNano())})

	// The model's internal length unit is cm and raw work-plane origin coordinates are taken
	// as-is (model units), whereas sketch_rectangle takes unit-bearing strings ("16 mm"). So
	// do all origin math in cm (CLI values are mm) to keep the loop, the origin shift, and the
	// rectangle consistent — a mismatch here de-centers and shrinks every section.
	const mmToCm = 0.1
	Rc, wc, tc := R*mmToCm, w*mmToCm, t*mmToCm
	sections := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		u := 2 * math.Pi * float64(i) / float64(n) // azimuth around the loop
		twist := u * turns                         // turns=0.5 → 0..π half-twist (Möbius); 0 → flat ring
		cu, su := math.Cos(u), math.Sin(u)
		ca, sa := math.Cos(twist), math.Sin(twist)
		// Cross-section plane is radial (its normal is the loop tangent). In-plane the band
		// is rotated by `twist`: width along (cosθ·r̂ + sinθ·ẑ), thickness perpendicular to it.
		wx, wy, wz := ca*cu, ca*su, sa   // width direction (sketch xAxis)
		tx, ty, tz := -sa*cu, -sa*su, ca // thickness direction (sketch yAxis)
		Cx, Cy := Rc*cu, Rc*su           // section center on the loop (z = 0), in cm
		// The ellipse is centered on the sketch origin, so the plane origin is the section center;
		// the rectangle is anchored at the sketch origin, so its plane origin is shifted to the
		// band corner (spanning [0,wc]×[0,tc]) to land centered on C.
		ox, oy, oz := Cx, Cy, 0.0
		if profile == "rect" {
			ox -= 0.5*wc*wx + 0.5*tc*tx
			oy -= 0.5*wc*wy + 0.5*tc*ty
			oz -= 0.5*wc*wz + 0.5*tc*tz
		}
		var wp struct {
			Index   int  `json:"index"`
			Healthy bool `json:"healthy"`
		}
		b.callJSON("create_work_plane", map[string]any{
			"kind":   "fixed-frame",
			"origin": []float64{ox, oy, oz},
			"xaxis":  []float64{wx, wy, wz},
			"yaxis":  []float64{tx, ty, tz},
		}, &wp)
		var sk struct {
			SketchIndex int `json:"sketchIndex"`
		}
		b.callJSON("create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
		b.addProfile(sk.SketchIndex, profile, w, t)
		sections = append(sections, map[string]any{"sketchIndex": sk.SketchIndex, "profileIndex": 0})
	}
	fmt.Printf("  placed %d cross-section profiles\n", len(sections))

	// One CLOSED loft through every section — the wrap from last→first carries the final
	// half-twist and closes the band onto itself (the W×T rectangle is 180°-symmetric, so the
	// seam is watertight).
	var lr struct {
		Feature string `json:"feature"`
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	_, isErr := b.callJSON("add_feature", map[string]any{
		"kind": "loft",
		"args": map[string]any{"sections": sections, "closed": closedLoft, "operation": "new"},
	}, &lr)
	status := "PASS"
	if isErr || !lr.Healthy {
		status = "FAIL"
	}
	fmt.Printf("  closed loft: %-5s %s %s\n", status, lr.Feature, lr.Reason)

	b.report()

	// Hide the datum overlays so the capture shows only the solid (36 work planes otherwise
	// draw as large translucent squares that bury the band).
	b.call("ui_set_object_visibility", map[string]any{
		"visibility": map[string]any{"workPlanes": false, "workAxes": false, "workPoints": false, "sketches": false},
	})

	// Frame and capture from a few angles so the half-twist reads clearly.
	b.call("execute_command", map[string]any{"id": "View.Home"})
	b.call("capture_viewport", map[string]any{"path": out})
	fmt.Printf("captured -> %s\n", out)
	stem := out[:len(out)-len(".png")]
	for _, v := range []struct {
		eye, up []float64
		suffix  string
	}{
		{[]float64{0, 0, 16}, []float64{0, 1, 0}, "-top"}, // up must NOT be parallel to view dir
		{[]float64{16, 0, 4}, []float64{0, 0, 1}, "-side"},
	} {
		b.call("set_camera", map[string]any{"eye": v.eye, "target": []float64{0, 0, 0}, "up": v.up, "fov": 0.6})
		p := stem + v.suffix + ".png"
		b.call("capture_viewport", map[string]any{"path": p})
		fmt.Printf("captured -> %s\n", p)
	}

	// Mesh-quality check (requested): normal-debug (front green / back red — reveals winding
	// flips, which a non-orientable Möbius surface is the worst case for) and per-triangle
	// colors. Captured from the Home iso so the whole band is in frame.
	b.call("execute_command", map[string]any{"id": "View.Home"})
	b.call("set_normal_debug", map[string]any{"on": true})
	b.call("capture_viewport", map[string]any{"path": stem + "-normals.png"})
	fmt.Printf("captured -> %s\n", stem+"-normals.png")
	b.call("set_normal_debug", map[string]any{"on": false})
	b.call("set_mesh_colors", map[string]any{"on": true, "perTriangle": true})
	b.call("capture_viewport", map[string]any{"path": stem + "-tris.png"})
	fmt.Printf("captured -> %s\n", stem+"-tris.png")
	b.call("set_mesh_colors", map[string]any{"on": false})
	if status != "PASS" {
		return fmt.Errorf("loft did not build healthy")
	}
	return nil
}

// barCheck extrudes a single w×t rectangle by the loop circumference (2πR) — the simplest
// possible solid built from the same rectangle primitive — and compares its volume to the
// exact w·t·L. It isolates whether the strip's volume shortfall is in the loft or upstream.
func barCheck(url string, R, w, t float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	mc := mcp.NewClient(&mcp.Implementation{Name: "mcpmobius-bar", Version: "0.1.0"}, nil)
	cs, err := mc.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer func() { _ = cs.Close() }()
	b := &builder{ctx, cs}
	b.call("close_all_documents", map[string]any{"force": true})
	b.call("create_document", map[string]any{"type": "part", "name": "bar-check"})
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	b.callJSON("create_sketch", map[string]any{"plane": "XY"}, &sk)
	b.callJSON("sketch_rectangle", map[string]any{
		"sketchIndex": sk.SketchIndex, "width": fmt.Sprintf("%g mm", w), "height": fmt.Sprintf("%g mm", t),
	}, nil)
	length := 2 * math.Pi * R // mm
	var lr struct {
		Healthy bool `json:"healthy"`
	}
	b.callJSON("add_feature", map[string]any{"kind": "extrude", "args": map[string]any{
		"sketchIndex": sk.SketchIndex, "profileIndex": 0, "distance": fmt.Sprintf("%g mm", length), "operation": "new",
	}}, &lr)
	var pp struct {
		Volume float64 `json:"volume"`
		Area   float64 `json:"area"`
	}
	b.callJSON("get_physical_properties", nil, &pp)
	wantV := w * t * length / 1000.0 // mm³ → cm³
	fmt.Printf("bar check: extrude %g×%g mm by %.2f mm\n", w, t, length)
	fmt.Printf("  volume=%.3f cm³ (exact w·t·L = %.3f cm³, ratio %.3f)\n", pp.Volume, wantV, pp.Volume/wantV)
	fmt.Printf("  area=%.3f cm²\n", pp.Area)
	return nil
}

// addProfile draws the cross-section on the sketch: a corner-anchored w×t rectangle, or an
// ellipse centered at the sketch origin with its major axis along the sketch x-axis (the band
// width direction), semi-axes w/2 × t/2 — the same w×t footprint, rounded.
func (b *builder) addProfile(sketchIndex int, profile string, w, t float64) {
	if profile == "ellipse" {
		b.call("add_sketch_entity", map[string]any{
			"sketchIndex": sketchIndex,
			"kind":        "ellipse",
			"points":      [][]float64{{0, 0}},
			"axis":        []float64{1, 0},
			"majorRadius": fmt.Sprintf("%g mm", w/2),
			"minorRadius": fmt.Sprintf("%g mm", t/2),
		})
		return
	}
	b.call("sketch_rectangle", map[string]any{
		"sketchIndex": sketchIndex,
		"width":       fmt.Sprintf("%g mm", w),
		"height":      fmt.Sprintf("%g mm", t),
	})
}

func (b *builder) report() {
	var pp struct {
		Volume float64 `json:"volume"`
		Area   float64 `json:"area"`
	}
	b.callJSON("get_physical_properties", nil, &pp)
	var tr struct {
		Bodies   int `json:"bodies"`
		Features []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"features"`
	}
	b.callJSON("get_model_tree", nil, &tr)
	fmt.Printf("  model: %d body, %d features, volume=%.3f cm³, area=%.3f cm²\n",
		tr.Bodies, len(tr.Features), pp.Volume, pp.Area)
}

func (b *builder) call(tool string, args map[string]any) { _, _ = b.callJSON(tool, args, nil) }

func (b *builder) callJSON(tool string, args map[string]any, v any) (string, bool) {
	res, err := b.cs.CallTool(b.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return err.Error(), true
	}
	text := ""
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text = tc.Text
			break
		}
	}
	if v != nil && text != "" {
		_ = json.Unmarshal([]byte(text), v)
	}
	return text, res.IsError
}
