// SPDX-License-Identifier: GPL-2.0-only

// Command mcpcornerdemo drives the LIVE oblikovati app to create one document PER fillet corner
// configuration so the result can be validated visually: each scenario builds a fresh box, fillets a
// chosen edge set with a chosen cornerType, frames the (4,3,2) corner, and captures a window PNG. The
// documents stay open as tabs.
//
// Usage: mcpcornerdemo [--url http://127.0.0.1:7800/mcp] [--dir /tmp/cornerdemo]
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
	dir := flag.String("dir", "/tmp/cornerdemo", "directory for the captured PNGs")
	flag.Parse()
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mcpcornerdemo:", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	cl := mcp.NewClient(&mcp.Implementation{Name: "mcpcornerdemo", Version: "0.1.0"}, nil)
	cs, err := cl.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: *url}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mcpcornerdemo connect:", err)
		os.Exit(1)
	}
	defer cs.Close()
	d := &driver{ctx: ctx, cs: cs, dir: *dir}
	d.call("close_all_documents", map[string]any{"force": true})

	// label, cornerType, edge selector (all top edges, or just the two meeting at the (4,3,2) corner)
	d.scenario("1-miter-allTopEdges", "miter", allTop)
	d.scenario("2-round-allTopEdges", "round", allTop)
	d.scenario("3-miter-oneCorner", "miter", twoAtCorner)
	d.scenario("4-round-oneCorner", "round", twoAtCorner)
	d.scenario("5-setback-oneCorner", "setback", twoAtCorner)
}

type driver struct {
	ctx context.Context
	cs  *mcp.ClientSession
	dir string
	seq int
}

type rawRef struct {
	Key   string    `json:"key"`
	Point []float64 `json:"point"`
}

// allTop selects the four top edges (z ≈ 2 cm).
func allTop(edges []rawRef) []string {
	var out []string
	for _, e := range edges {
		if len(e.Point) == 3 && e.Point[2] > 1.99 {
			out = append(out, e.Key)
		}
	}
	return out
}

// twoAtCorner selects the two top edges meeting at the (4,3,2) corner: the top-X edge (mid 2,3,2)
// and the top-Y edge (mid 4,1.5,2).
func twoAtCorner(edges []rawRef) []string {
	var out []string
	for _, e := range edges {
		if len(e.Point) != 3 || e.Point[2] <= 1.99 {
			continue
		}
		near := func(a, b float64) bool { return a-b < 0.05 && b-a < 0.05 }
		if (near(e.Point[1], 3) && e.Point[0] < 3.9) || (near(e.Point[0], 4) && e.Point[1] < 2.9) {
			out = append(out, e.Key)
		}
	}
	return out
}

func (d *driver) scenario(label, corner string, pick func([]rawRef) []string) {
	d.box()
	edges := pick(d.edges())
	status := d.fillet(edges, "4 mm", corner)
	fmt.Printf("%-22s cornerType=%-8s edges=%d %s; spheres=%d\n", label, corner, len(edges), status, d.sphereFaces())
	d.capture(label)
}

var docNames = []string{"miter-all", "round-all", "miter-corner", "round-corner", "setback-corner"}

func (d *driver) box() {
	d.call("create_document", map[string]any{"type": "part", "name": docNames[d.seq]})
	d.seq++
	d.call("create_sketch", map[string]any{"plane": "XY"})
	d.call("sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"})
	d.callJSON("add_feature", map[string]any{"kind": "extrude",
		"args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"}}, &struct{}{})
}

func (d *driver) fillet(edgeRefs []string, radius, corner string) string {
	var r struct {
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	d.callJSON("add_feature", map[string]any{"kind": "fillet",
		"args": map[string]any{"edgeRefs": edgeRefs, "radius": radius, "cornerType": corner}}, &r)
	if !r.Healthy {
		if r.Reason == "" {
			return "unhealthy(no reason)"
		}
		return "SICK: " + r.Reason
	}
	return "healthy"
}

func (d *driver) capture(label string) {
	d.call("set_camera", map[string]any{"eye": []float64{6.5, 5.5, 4.2}, "target": []float64{3.4, 2.6, 1.7}, "up": []float64{0, 0, 1}, "fov": 0.55})
	d.call("set_mesh_colors", map[string]any{"on": true, "perTriangle": false})
	out := d.dir + "/" + label + ".png"
	d.call("capture_window", map[string]any{"path": out})
	fmt.Printf("    -> %s\n", out)
}

func (d *driver) sphereFaces() int {
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Kind string `json:"kind"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	n := 0
	if d.callJSON("get_reference_keys", nil, &rk) && len(rk.Bodies) > 0 {
		for _, f := range rk.Bodies[0].Faces {
			if f.Kind == "sphere" {
				n++
			}
		}
	}
	return n
}

func (d *driver) edges() []rawRef {
	var rk struct {
		Bodies []struct {
			Edges []rawRef `json:"edges"`
		} `json:"bodies"`
	}
	if !d.callJSON("get_reference_keys", nil, &rk) || len(rk.Bodies) == 0 {
		return nil
	}
	return rk.Bodies[0].Edges
}

func (d *driver) call(tool string, a map[string]any) {
	_, _ = d.cs.CallTool(d.ctx, &mcp.CallToolParams{Name: tool, Arguments: a})
}

func (d *driver) callJSON(tool string, a map[string]any, v any) bool {
	res, err := d.cs.CallTool(d.ctx, &mcp.CallToolParams{Name: tool, Arguments: a})
	if err != nil || res.IsError {
		return false
	}
	for _, ct := range res.Content {
		if tc, ok := ct.(*mcp.TextContent); ok {
			return json.Unmarshal([]byte(tc.Text), v) == nil
		}
	}
	return false
}
