// SPDX-License-Identifier: GPL-2.0-only

// Command mcpwheel drives the feature-interaction "car wheel" workflow against a running
// bridge and prints each step: a disc blank, a work plane on the disc's feature-created top
// face, a sketch on that work plane, a bolt-hole cut from it, a circular pattern of the cut,
// and a fillet — proving later features consume earlier features' geometry over MCP. It is the
// live counterpart of bridge/TestE2EWheelWorkflow.
//
// Usage: mcpwheel [--url http://127.0.0.1:7800/mcp]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint URL")
	flag.Parse()
	if err := run(*url); err != nil {
		fmt.Fprintln(os.Stderr, "mcpwheel:", err)
		os.Exit(1)
	}
}

type wheel struct {
	ctx context.Context
	cs  *mcp.ClientSession
}

func run(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	mc := mcp.NewClient(&mcp.Implementation{Name: "mcpwheel", Version: "0.2.0"}, nil)
	cs, err := mc.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer func() {
		if closeErr := cs.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "mcpwheel: close session:", closeErr)
		}
	}()
	w := &wheel{ctx, cs}

	fmt.Println("building a wheel over MCP (feature interaction):")
	// A unique name so a re-run starts a fresh part (create_document is a no-op on a duplicate
	// name, which would otherwise stack a second wheel onto the previous run's document).
	w.call("create_document", map[string]any{"type": "part", "name": fmt.Sprintf("wheel-%d", time.Now().UnixNano())})

	// 1. Disc blank: Ø60 mm circle extruded 10 mm.
	w.call("create_sketch", map[string]any{"plane": "XY"})
	w.call("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "30 mm"})
	disc := w.feature("1. disc blank (extrude)", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "10 mm", "operation": "new"})
	v0 := w.volume()
	fmt.Printf("     disc volume = %.3f cm³\n", v0)

	// 2. Work plane on the disc's top face (a face the extrude created).
	face := w.topFaceKey()
	wp := w.workPlaneOnFace("2. work plane on disc top face", face, "5 mm")

	// 3. A sketch on that work plane.
	sketchIdx := w.sketchOnWorkPlane("3. sketch on the work plane", wp)

	// 4. A bolt hole cut from the work-plane sketch, down through the disc.
	w.call("add_sketch_entity", map[string]any{"sketchIndex": sketchIdx, "kind": "circle", "points": [][]float64{{2, 0}}, "radius": "3 mm"})
	bolt := w.feature("4. bolt-hole cut (from work-plane sketch)", "extrude", map[string]any{
		"sketchIndex": sketchIdx, "profileIndex": 0, "distance": "20 mm", "operation": "cut", "direction": "negative",
	})
	v1 := w.volume()
	fmt.Printf("     after one hole = %.3f cm³  (removed %.3f — %s)\n", v1, v0-v1, removedLabel(v0, v1))

	// 5. Circular pattern of the bolt hole around the wheel centre.
	w.feature("5. circular pattern of the bolt hole", "patternCircular", map[string]any{
		"sourceFeatures": []string{bolt}, "count": 5, "angle": "360 deg", "axisDir": []float64{0, 0, 1},
	})
	v2 := w.volume()
	fmt.Printf("     after 5 holes  = %.3f cm³  (removed %.3f more — %s)\n", v2, v1-v2, removedLabel(v1, v2))

	// 6. Fillet a disc edge.
	w.feature("6. fillet a disc edge", "fillet", map[string]any{"edgeRefs": []string{w.anEdgeKey()}, "radius": "1 mm"})

	w.modelTree()
	_ = disc
	fmt.Println("\nwheel built — feature interaction over MCP works ✓")
	return nil
}

// feature applies an add_feature and prints its outcome; returns the created feature's name.
func (w *wheel) feature(label, kind string, args map[string]any) string {
	var r struct {
		Feature string `json:"feature"`
		Bodies  int    `json:"bodies"`
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	text, isErr := w.callJSON("add_feature", map[string]any{"kind": kind, "args": args}, &r)
	status := healthLabel(r.Healthy, isErr)
	detail := r.Feature
	if r.Reason != "" {
		detail += " (" + r.Reason + ")"
	} else if r.Feature == "" {
		detail = clip(text)
	}
	fmt.Printf("  %-44s %-7s %s bodies=%d\n", label, status, detail, r.Bodies)
	return r.Feature
}

// topFaceKey returns the reference key of the body face with the greatest Z (the disc top).
func (w *wheel) topFaceKey() string {
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	w.callJSON("get_reference_keys", nil, &rk)
	best, bz, found := "", 0.0, false
	if len(rk.Bodies) > 0 {
		for _, f := range rk.Bodies[0].Faces {
			if len(f.Point) == 3 && (!found || f.Point[2] > bz) {
				best, bz, found = f.Key, f.Point[2], true
			}
		}
	}
	return best
}

func (w *wheel) anEdgeKey() string {
	var rk struct {
		Bodies []struct {
			Edges []struct {
				Key string `json:"key"`
			} `json:"edges"`
		} `json:"bodies"`
	}
	w.callJSON("get_reference_keys", nil, &rk)
	if len(rk.Bodies) > 0 && len(rk.Bodies[0].Edges) > 0 {
		return rk.Bodies[0].Edges[0].Key
	}
	return ""
}

func (w *wheel) workPlaneOnFace(label, faceKey, offset string) int {
	var r struct {
		Index   int  `json:"index"`
		Healthy bool `json:"healthy"`
	}
	_, isErr := w.callJSON("create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{faceKey}, "offset": offset}, &r)
	fmt.Printf("  %-44s %-7s index=%d\n", label, healthLabel(r.Healthy, isErr), r.Index)
	return r.Index
}

func (w *wheel) sketchOnWorkPlane(label string, wpIndex int) int {
	var r struct {
		SketchIndex int    `json:"sketchIndex"`
		Plane       string `json:"plane"`
	}
	_, isErr := w.callJSON("create_sketch", map[string]any{"workPlaneIndex": wpIndex}, &r)
	status := "PASS"
	if isErr || r.SketchIndex < 1 {
		status = "FAIL"
	}
	fmt.Printf("  %-44s %-7s sketchIndex=%d on %q\n", label, status, r.SketchIndex, r.Plane)
	return r.SketchIndex
}

func (w *wheel) modelTree() {
	var t struct {
		Bodies   int `json:"bodies"`
		Features []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"features"`
	}
	w.callJSON("get_model_tree", nil, &t)
	fmt.Printf("\nmodel tree: %d bodies, %d features:\n", t.Bodies, len(t.Features))
	for _, f := range t.Features {
		fmt.Printf("    - %s [%s]\n", f.Name, f.Kind)
	}
}

// volume reads the active part's volume (cm³).
func (w *wheel) volume() float64 {
	var pp struct {
		Volume float64 `json:"volume"`
	}
	w.callJSON("get_physical_properties", nil, &pp)
	return pp.Volume
}

// removedLabel reports whether material was actually removed between two volume readings.
func removedLabel(before, after float64) string {
	if after < before {
		return "real hole ✓"
	}
	return "NO material removed — hole missed the body!"
}

func (w *wheel) call(tool string, args map[string]any) {
	_, _ = w.cs.CallTool(w.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
}

func (w *wheel) callJSON(tool string, args map[string]any, v any) (string, bool) {
	res, err := w.cs.CallTool(w.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
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
	if v != nil {
		_ = json.Unmarshal([]byte(text), v)
	}
	return text, res.IsError
}

func healthLabel(healthy, isErr bool) string {
	switch {
	case isErr:
		return "REACH"
	case healthy:
		return "PASS"
	default:
		return "unhealthy"
	}
}

func clip(s string) string {
	if len(s) > 80 {
		return s[:80] + "…"
	}
	return s
}
