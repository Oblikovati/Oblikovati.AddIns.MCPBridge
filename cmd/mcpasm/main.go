// SPDX-License-Identifier: GPL-2.0-only

// Command mcpasm builds an assembly with one box component placed in an N×N grid (all copies share
// one component definition) and captures the viewport — the visual acceptance test for render
// instancing (ADR-0038): the unique mesh is tessellated/uploaded once and drawn at every placement.
// Usage: mcpasm [--grid 4] [--pitch 8] [--out /tmp/asm.png]
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

type builder struct {
	ctx context.Context
	cs  *mcp.ClientSession
}

func (b *builder) call(tool string, args map[string]any, out any) {
	res, err := b.cs.CallTool(b.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", tool, err)
		os.Exit(1)
	}
	if out != nil {
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				_ = json.Unmarshal([]byte(tc.Text), out)
				break
			}
		}
	}
}

// translation returns a 16-cell row-major translation matrix (model units = cm).
func translation(x, y, z float64) []float64 {
	return []float64{1, 0, 0, x, 0, 1, 0, y, 0, 0, 1, z, 0, 0, 0, 1}
}

// buildMobius builds an ellipse Möbius strip in the active part (36 fixed-frame sections, closed
// loft), so the instancing benchmark stresses a heavy component (~14k triangles), not a 12-tri box.
// Mirrors cmd/mcpmobius (--profile ellipse) in cm model units: ring R=3, ellipse 1.6×0.2.
func (b *builder) buildMobius() {
	const n = 36
	const R, wMM, tMM = 3.0, 16.0, 2.0
	sections := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		u := 2 * math.Pi * float64(i) / float64(n)
		a := u / 2
		cu, su := math.Cos(u), math.Sin(u)
		ca, sa := math.Cos(a), math.Sin(a)
		var wp struct {
			Index int `json:"index"`
		}
		b.call("create_work_plane", map[string]any{
			"kind":   "fixed-frame",
			"origin": []float64{R * cu, R * su, 0},
			"xaxis":  []float64{ca * cu, ca * su, sa},
			"yaxis":  []float64{-sa * cu, -sa * su, ca},
		}, &wp)
		var sk struct {
			SketchIndex int `json:"sketchIndex"`
		}
		b.call("create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
		b.call("add_sketch_entity", map[string]any{
			"sketchIndex": sk.SketchIndex, "kind": "ellipse", "points": [][]float64{{0, 0}},
			"axis": []float64{1, 0}, "majorRadius": fmt.Sprintf("%g mm", wMM/2), "minorRadius": fmt.Sprintf("%g mm", tMM/2),
		}, nil)
		sections = append(sections, map[string]any{"sketchIndex": sk.SketchIndex, "profileIndex": 0})
	}
	b.call("add_feature", map[string]any{"kind": "loft", "args": map[string]any{
		"sections": sections, "closed": true, "operation": "new"}}, nil)
}

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint")
	grid := flag.Int("grid", 4, "grid is grid×grid placements of the one component")
	pitch := flag.Float64("pitch", 8, "grid spacing (cm)")
	out := flag.String("out", "/tmp/oblikovati-asm.png", "captured PNG")
	component := flag.String("component", "box", "component to instance: box | mobius")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()
	mc := mcp.NewClient(&mcp.Implementation{Name: "mcpasm", Version: "0.1.0"}, nil)
	cs, err := mc.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: *url}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer cs.Close()
	b := &builder{ctx, cs}

	b.call("close_all_documents", map[string]any{"force": true}, nil)
	b.call("create_document", map[string]any{"type": "part", "name": *component}, nil)
	if *component == "mobius" {
		b.buildMobius() // a heavy (~14k-tri) ellipse Möbius component
	} else {
		b.call("create_sketch", map[string]any{"plane": "XY"}, nil)
		b.call("sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "40 mm"}, nil)
		b.call("add_feature", map[string]any{"kind": "extrude", "args": map[string]any{
			"sketchIndex": 0, "profileIndex": 0, "distance": "40 mm", "operation": "new"}}, nil)
	}

	var docs struct {
		Documents []struct {
			ID   uint64 `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"documents"`
	}
	b.call("list_documents", nil, &docs)
	var boxID uint64
	for _, d := range docs.Documents {
		if d.Type == "part" {
			boxID = d.ID
		}
	}
	if boxID == 0 {
		fmt.Fprintln(os.Stderr, "no part document found")
		os.Exit(1)
	}

	// An assembly with the box placed grid×grid times — all copies share the one definition.
	b.call("create_document", map[string]any{"type": "assembly", "name": "asm"}, nil)
	n := *grid
	off := float64(n-1) * *pitch / 2
	var first struct {
		Occurrence struct {
			ID uint64 `json:"id"`
		} `json:"occurrence"`
	}
	b.call("place_component", map[string]any{
		"document": boxID, "name": "box:1", "transform": translation(-off, -off, 0)}, &first)
	count := 1
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == 0 && j == 0 {
				continue
			}
			count++
			b.call("place_component_copy", map[string]any{
				"source":    first.Occurrence.ID,
				"name":      fmt.Sprintf("box:%d", count),
				"transform": translation(float64(i)**pitch-off, float64(j)**pitch-off, 0),
			}, nil)
		}
	}
	fmt.Printf("placed %d copies of one box component (grid %d×%d)\n", count, n, n)

	b.call("execute_command", map[string]any{"id": "View.Home"}, nil)
	b.call("capture_viewport", map[string]any{"path": *out}, nil)
	fmt.Printf("captured -> %s\n", *out)
}
