// SPDX-License-Identifier: GPL-2.0-only

// Command mcprimfillet drives the LIVE oblikovati app over the MCP bridge to exercise the CIRCULAR
// RIM fillet — rounding the top edge of a cylinder/boss into a toroidal band (the closed-rim
// curved-adjacent fillet). For each scenario it sketches a circle, extrudes a cylinder, fillets the
// top rim, and captures a window PNG, logging the add_feature health and the edge/face/volume deltas
// (a real rim fillet adds a torus face and removes the rim notch).
//
// Usage: mcprimfillet [--url http://127.0.0.1:7800/mcp] [--dir /tmp/rimfillet]
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
	dir := flag.String("dir", "/tmp/rimfillet", "directory for the captured PNGs")
	flag.Parse()
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mcprimfillet:", err)
		os.Exit(1)
	}
	if err := run(*url, *dir); err != nil {
		fmt.Fprintln(os.Stderr, "mcprimfillet:", err)
		os.Exit(1)
	}
}

type ref struct {
	key string
	p   [3]float64
}

type driver struct {
	ctx context.Context
	cs  *mcp.ClientSession
	dir string
	seq int
}

func run(url, dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cl := mcp.NewClient(&mcp.Implementation{Name: "mcprimfillet", Version: "0.1.0"}, nil)
	cs, err := cl.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer cs.Close()
	d := &driver{ctx: ctx, cs: cs, dir: dir}
	d.call("close_all_documents", map[string]any{"force": true})

	// radiusMM, heightMM, filletMM
	d.scenario("1-cylinder-r20-h30-f5", 20, 30, "5 mm")
	d.scenario("2-tall-r10-h40-f3", 10, 40, "3 mm")
	d.scenario("3-flat-r30-h10-f8", 30, 10, "8 mm")
	return nil
}

// scenario sketches a circle, extrudes a cylinder, fillets its top rim, and reports + captures.
func (d *driver) scenario(label string, radiusMM, heightMM float64, fillet string) {
	d.cylinder(radiusMM, heightMM)
	topZ := heightMM / 10 // mm → cm (the database unit)
	rim := nearestEdge(d.edges(), [3]float64{0, 0, topZ})
	if rim == nil {
		d.capture(label, topZ, "no rim edge")
		return
	}
	beforeE, beforeF, beforeV := len(d.edges()), len(d.faces()), d.volume()
	status := d.fillet(rim.key, fillet)
	fmt.Printf("%-26s rim mid=(% .2f % .2f % .2f) %s; edges %d→%d faces %d→%d vol %.4f→%.4f; torus=%d\n",
		label, rim.p[0], rim.p[1], rim.p[2], status, beforeE, len(d.edges()), beforeF, len(d.faces()), beforeV, d.volume(), d.torusFaces())
	d.capture(label, topZ, status)
}

// cylinder creates a fresh part with a circle extruded to a cylinder (radius/height in mm).
func (d *driver) cylinder(radiusMM, heightMM float64) {
	d.seq++
	d.call("create_document", map[string]any{"type": "part", "name": fmt.Sprintf("rim-%d", d.seq)})
	d.call("create_sketch", map[string]any{"plane": "XY"})
	d.call("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "circle",
		"points": [][]float64{{0, 0}}, "radius": fmt.Sprintf("%g mm", radiusMM)})
	d.callJSON("add_feature", map[string]any{"kind": "extrude",
		"args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": fmt.Sprintf("%g mm", heightMM), "operation": "new"}}, &struct{}{})
}

// fillet rounds the rim edge and returns a status string.
func (d *driver) fillet(rimKey, radius string) string {
	var r struct {
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	d.callJSON("add_feature", map[string]any{"kind": "fillet", "args": map[string]any{"edgeRefs": []string{rimKey}, "radius": radius}}, &r)
	if !r.Healthy {
		if r.Reason == "" {
			return "unhealthy(no reason)"
		}
		return "SICK: " + r.Reason
	}
	return "healthy"
}

// capture frames on the rounded top rim and writes a window PNG.
func (d *driver) capture(label string, topZ float64, note string) {
	eye := []float64{topZ * 3, topZ * 3, topZ * 2.2}
	d.call("set_camera", map[string]any{"eye": eye, "target": []float64{0, 0, topZ}, "up": []float64{0, 0, 1}, "fov": 0.6})
	d.call("set_mesh_colors", map[string]any{"on": true, "perTriangle": false})
	out := d.dir + "/" + label + ".png"
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

func (d *driver) edges() []ref { return d.topo(func(b bodyTopo) []rawRef { return b.Edges }) }
func (d *driver) faces() []ref { return d.topo(func(b bodyTopo) []rawRef { return b.Faces }) }

func (d *driver) torusFaces() int {
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
			if f.Kind == "torus" {
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
type bodyTopo struct {
	Faces []rawRef `json:"faces"`
	Edges []rawRef `json:"edges"`
}

func (d *driver) topo(pick func(bodyTopo) []rawRef) []ref {
	var rk struct {
		Bodies []bodyTopo `json:"bodies"`
	}
	if !d.callJSON("get_reference_keys", nil, &rk) || len(rk.Bodies) == 0 {
		return nil
	}
	var out []ref
	for _, e := range pick(rk.Bodies[0]) {
		if len(e.Point) == 3 {
			out = append(out, ref{key: e.Key, p: [3]float64{e.Point[0], e.Point[1], e.Point[2]}})
		}
	}
	return out
}

func nearestEdge(e []ref, target [3]float64) *ref {
	var best *ref
	bestD := 0.0
	for i := range e {
		dx, dy, dz := e[i].p[0]-target[0], e[i].p[1]-target[1], e[i].p[2]-target[2]
		dd := dx*dx + dy*dy + dz*dz
		if best == nil || dd < bestD {
			best, bestD = &e[i], dd
		}
	}
	return best
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
