// SPDX-License-Identifier: GPL-2.0-only

// Command mcpfan builds a parametric PC fan over a running bridge — a complex, multi-feature
// modeling test (square frame, rounded corners, central bore + hub, a circular pattern of
// blades, and four mounting holes) modelled on parametric-fan.scad. It prints each step and
// the running volume so material changes are visible. The live counterpart of
// bridge/TestE2EParametricFan; watch the viewport as it builds.
//
// Usage: mcpfan [--url http://127.0.0.1:7800/mcp]
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
		fmt.Fprintln(os.Stderr, "mcpfan:", err)
		os.Exit(1)
	}
}

type fan struct {
	ctx context.Context
	cs  *mcp.ClientSession
}

func run(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	mc := mcp.NewClient(&mcp.Implementation{Name: "mcpfan", Version: "0.2.0"}, nil)
	cs, err := mc.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer func() {
		if closeErr := cs.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "mcpfan: close session:", closeErr)
		}
	}()
	f := &fan{ctx, cs}

	fmt.Println("building a parametric fan over MCP (watch the viewport):")
	f.call("create_document", map[string]any{"type": "part", "name": fmt.Sprintf("fan-%d", time.Now().UnixNano())})

	// 1. Frame: a centred 5×5 cm square, 1.5 cm thick.
	s := f.sketch()
	f.call("add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {2.5, 2.5}}})
	f.step("1. frame (5x5x1.5 extrude)", "extrude", map[string]any{"sketchIndex": s, "profileIndex": 0, "distance": "15 mm", "operation": "new"})

	// 2. Round the four vertical corner edges.
	if corners := f.cornerEdges(); len(corners) >= 4 {
		f.step("2. round 4 corners (fillet r5)", "fillet", map[string]any{"edgeRefs": corners[:4], "radius": "5 mm"})
	} else {
		fmt.Printf("  %-40s SKIP found %d corner edges\n", "2. round 4 corners", len(corners))
	}

	// 3. Central bore Ø47.
	s = f.sketch()
	f.call("add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "23.5 mm"})
	f.step("3. central bore (Ø47 cut)", "extrude", map[string]any{"sketchIndex": s, "profileIndex": 0, "distance": "30 mm", "operation": "cut", "direction": "symmetric"})

	// 4. Hub Ø13.5.
	s = f.sketch()
	f.call("add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "6.75 mm"})
	f.step("4. hub (Ø13.5 join)", "extrude", map[string]any{"sketchIndex": s, "profileIndex": 0, "distance": "15 mm", "operation": "join"})

	// 5. One TWISTED blade, LOFTED between a root profile on XY and a 20°-twisted tip profile on
	// a work plane at z=14 mm (exercises multiple work planes, multiple sketches, and loft).
	root := f.sketch()
	f.call("add_sketch_entity", map[string]any{"sketchIndex": root, "kind": "rectangle", "points": [][]float64{{0.6, -0.08}, {2.35, 0.08}}})
	var tipWP struct {
		Index int `json:"index"`
	}
	f.callJSON("create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "14 mm"}, &tipWP)
	var tipSk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	f.callJSON("create_sketch", map[string]any{"workPlaneIndex": tipWP.Index}, &tipSk)
	f.call("add_sketch_entity", map[string]any{"sketchIndex": tipSk.SketchIndex, "kind": "rectangle", "variant": "threePoint",
		"points": [][]float64{{0.591, 0.130}, {2.236, 0.729}, {2.182, 0.879}}})
	blade := f.step("5. one twisted blade (loft join)", "loft", map[string]any{
		"sections":  []map[string]any{{"sketchIndex": root, "profileIndex": 0}, {"sketchIndex": tipSk.SketchIndex, "profileIndex": 0}},
		"operation": "join",
	})

	// 6. Seven blades via a circular pattern of the join.
	f.step("6. 7 blades (circular pattern)", "patternCircular", map[string]any{
		"sourceFeatures": []string{blade}, "count": 7, "angle": "360 deg", "axisDir": []float64{0, 0, 1},
	})

	// 7. Four mounting holes.
	s = f.sketch()
	for _, c := range [][2]float64{{2, 2}, {-2, 2}, {-2, -2}, {2, -2}} {
		f.call("add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "circle", "points": [][]float64{{c[0], c[1]}}, "radius": "1.7 mm"})
	}
	for pi := 0; pi < 4; pi++ {
		f.step(fmt.Sprintf("7. mounting hole %d (cut)", pi+1), "extrude", map[string]any{"sketchIndex": s, "profileIndex": pi, "distance": "30 mm", "operation": "cut", "direction": "symmetric"})
	}

	f.tree()
	fmt.Println("\nfan built ✓")
	return nil
}

// step applies a feature, prints its outcome + the running volume, and returns its name.
func (f *fan) step(label, kind string, args map[string]any) string {
	var r struct {
		Feature string `json:"feature"`
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	_, isErr := f.callJSON("add_feature", map[string]any{"kind": kind, "args": args}, &r)
	status := "PASS"
	if isErr || !r.Healthy {
		status = "FAIL"
	}
	detail := r.Reason
	if detail == "" {
		detail = r.Feature
	}
	fmt.Printf("  %-40s %-5s %-14s vol=%.3f cm³\n", label, status, detail, f.volume())
	return r.Feature
}

func (f *fan) cornerEdges() []string {
	var rk struct {
		Bodies []struct {
			Edges []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"edges"`
		} `json:"bodies"`
	}
	f.callJSON("get_reference_keys", nil, &rk)
	var out []string
	if len(rk.Bodies) > 0 {
		for _, e := range rk.Bodies[0].Edges {
			if len(e.Point) == 3 && abs(e.Point[0]) > 2.0 && abs(e.Point[1]) > 2.0 {
				out = append(out, e.Key)
			}
		}
	}
	return out
}

func (f *fan) sketch() int {
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	f.callJSON("create_sketch", map[string]any{"plane": "XY"}, &sk)
	return sk.SketchIndex
}

func (f *fan) volume() float64 {
	var pp struct {
		Volume float64 `json:"volume"`
	}
	f.callJSON("get_physical_properties", nil, &pp)
	return pp.Volume
}

func (f *fan) tree() {
	var t struct {
		Bodies   int `json:"bodies"`
		Features []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"features"`
	}
	f.callJSON("get_model_tree", nil, &t)
	fmt.Printf("\nmodel tree: %d body, %d features\n", t.Bodies, len(t.Features))
}

func (f *fan) call(tool string, args map[string]any) { _, _ = f.callJSONRaw(tool, args) }

func (f *fan) callJSONRaw(tool string, args map[string]any) (*mcp.CallToolResult, error) {
	return f.cs.CallTool(f.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
}

func (f *fan) callJSON(tool string, args map[string]any, v any) (string, bool) {
	res, err := f.callJSONRaw(tool, args)
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

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
