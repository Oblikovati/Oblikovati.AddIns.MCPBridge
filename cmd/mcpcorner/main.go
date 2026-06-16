// SPDX-License-Identifier: GPL-2.0-only

// Command mcpcorner drives the LIVE oblikovati app over the MCP bridge to exercise the fillet
// CORNER strategies (miter / round / setback): for each it builds a 4x3x2 box, fillets all four top
// edges at once with that cornerType, captures a window PNG, and logs the add_feature health plus the
// sphere-face count (a round corner adds a sphere octant per top corner; a miter adds none).
//
// Usage: mcpcorner [--url http://127.0.0.1:7800/mcp] [--dir /tmp/corner]
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
	dir := flag.String("dir", "/tmp/corner", "directory for the captured PNGs")
	flag.Parse()
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mcpcorner:", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cl := mcp.NewClient(&mcp.Implementation{Name: "mcpcorner", Version: "0.1.0"}, nil)
	cs, err := cl.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: *url}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mcpcorner connect:", err)
		os.Exit(1)
	}
	defer cs.Close()
	d := &driver{ctx: ctx, cs: cs, dir: *dir}
	d.call("close_all_documents", map[string]any{"force": true})
	for _, corner := range []string{"miter", "round", "setback"} {
		d.scenario(corner)
	}
}

type driver struct {
	ctx context.Context
	cs  *mcp.ClientSession
	dir string
	seq int
}

// scenario builds a fresh box, fillets the four top edges with cornerType, and reports + captures.
func (d *driver) scenario(corner string) {
	d.box()
	top := d.topEdges()
	beforeV := d.volume()
	status := d.fillet(top, "3 mm", corner)
	fmt.Printf("%-8s top-edges=%d %s; spheres=%d vol %.4f→%.4f\n",
		corner, len(top), status, d.sphereFaces(), beforeV, d.volume())
	d.capture(corner, status)
}

// box creates a fresh part with a 40x30x20 mm block (4x3x2 cm).
func (d *driver) box() {
	d.seq++
	d.call("create_document", map[string]any{"type": "part", "name": fmt.Sprintf("corner-%d", d.seq)})
	d.call("create_sketch", map[string]any{"plane": "XY"})
	d.call("sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"})
	d.callJSON("add_feature", map[string]any{"kind": "extrude",
		"args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"}}, &struct{}{})
}

// topEdges returns the reference keys of the four edges at the top (z ≈ 2 cm).
func (d *driver) topEdges() []string {
	var keys []string
	for _, e := range d.edges() {
		if len(e.Point) == 3 && e.Point[2] > 1.99 {
			keys = append(keys, e.Key)
		}
	}
	return keys
}

// fillet rounds the edges with the given cornerType and returns a status string.
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

func (d *driver) capture(corner, note string) {
	d.call("set_camera", map[string]any{"eye": []float64{6, 5, 4}, "target": []float64{2, 1.5, 1}, "up": []float64{0, 0, 1}, "fov": 0.6})
	d.call("set_mesh_colors", map[string]any{"on": true, "perTriangle": false})
	out := d.dir + "/" + corner + ".png"
	d.call("capture_window", map[string]any{"path": out})
	fmt.Printf("    -> %s\n", out)
}

func (d *driver) volume() float64 {
	var pp struct {
		Volume float64 `json:"volume"`
	}
	d.callJSON("get_physical_properties", nil, &pp)
	return pp.Volume
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

type rawRef struct {
	Key   string    `json:"key"`
	Point []float64 `json:"point"`
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
