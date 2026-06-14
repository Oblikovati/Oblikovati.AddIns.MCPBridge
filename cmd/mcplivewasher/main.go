// SPDX-License-Identifier: GPL-2.0-only

// Command mcplivewasher drives the NopSCADlib washer workflow against a *running*
// oblikovati-mcp-bridge host over HTTP/SSE — the live counterpart of the in-proc
// bridge/nopscad_washer_test.go. It creates a fresh part, declares parameters,
// fully constrains an annulus cross-section (0 DOF), revolves it into a ring, then
// resizes via a parameter edit and checks the volume tracks — exercising the live
// parameter DAG, solver, revolve feature and recompute chain end to end.
//
// Usage: mcplivewasher [--url http://127.0.0.1:7800/mcp]
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
	flag.Parse()
	if err := run(*url); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("PASS: live washer parametric workflow")
}

func run(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "mcplivewasher", Version: "0.1.0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer func() {
		if closeErr := cs.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "mcplivewasher: close session:", closeErr)
		}
	}()
	c := &caller{ctx: ctx, cs: cs}

	// Fresh part so we don't build on the demo document.
	var docRes struct {
		ID uint64 `json:"id"`
	}
	c.json("create_document", map[string]any{"type": "part", "name": "washer-live"}, &docRes)
	c.json("activate_document", map[string]any{"id": docRes.ID}, nil)

	c.json("add_parameter", map[string]any{"name": "od", "expression": "7 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "id", "expression": "3.1 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "thickness", "expression": "0.5 mm"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)

	rect := c.ids(map[string]any{"sketchIndex": 0, "kind": "rectangle",
		"points": [][]float64{{0.155, 0}, {0.35, 0.05}}})
	bl, br, tr, tl := rect[1], rect[2], rect[3], rect[4]
	o := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]

	con := func(kind string, ents ...uint64) {
		c.json("add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("horizontal", bl, br)
	con("horizontal", tl, tr)
	con("vertical", bl, tl)
	con("vertical", br, tr)
	con("ground", o)
	con("horizontal", o, bl)

	dim := func(kind, expr string, ents ...uint64) {
		c.json("add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents, "expression": expr}, nil)
	}
	dim("distance", "id / 2", o, bl)
	dim("distance", "(od - id) / 2", bl, br)
	dim("distance", "thickness", bl, tl)

	var solve struct {
		DOF int `json:"dof"`
	}
	c.json("solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	fmt.Printf("sketch DOF = %d (want 0)\n", solve.DOF)
	if solve.DOF != 0 {
		return fmt.Errorf("sketch not fully constrained: dof=%d", solve.DOF)
	}

	var feat struct {
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	c.json("add_feature", map[string]any{"kind": "revolve", "args": map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "axisRef": "origin/axis/y", "angle": "360 deg",
	}}, &feat)
	if !feat.Healthy {
		return fmt.Errorf("revolve unhealthy: %s", feat.Reason)
	}

	want := func(odMM, idMM, thMM float64) float64 {
		R, r, h := odMM/20, idMM/20, thMM/10
		return math.Pi * (R*R - r*r) * h
	}
	if err := c.checkVolume("od=7", want(7, 3.1, 0.5)); err != nil {
		return err
	}

	c.json("set_parameter", map[string]any{"name": "od", "expression": "10 mm"}, nil)
	if err := c.checkVolume("od=10 (resized)", want(10, 3.1, 0.5)); err != nil {
		return err
	}
	return c.err
}

type caller struct {
	ctx context.Context
	cs  *mcp.ClientSession
	err error
}

// json calls a tool and unmarshals the first text content into v (nil to ignore).
func (c *caller) json(name string, args map[string]any, v any) {
	if c.err != nil {
		return
	}
	res, err := c.cs.CallTool(c.ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		c.err = fmt.Errorf("%s: %w", name, err)
		return
	}
	text := firstText(res)
	if res.IsError {
		c.err = fmt.Errorf("%s tool error: %s", name, text)
		return
	}
	if v != nil && text != "" {
		if err := json.Unmarshal([]byte(text), v); err != nil {
			c.err = fmt.Errorf("%s: decode %q: %w", name, text, err)
		}
	}
}

// ids returns [entityId, point/entity ids...] from an add_sketch_entity reply.
func (c *caller) ids(args map[string]any) []uint64 {
	var r struct {
		EntityID  uint64   `json:"entityId"`
		PointIDs  []uint64 `json:"pointIds"`
		EntityIDs []uint64 `json:"entityIds"`
	}
	c.json("add_sketch_entity", args, &r)
	if len(r.PointIDs) > 0 {
		return append([]uint64{r.EntityID}, r.PointIDs...)
	}
	return append([]uint64{r.EntityID}, r.EntityIDs...)
}

func (c *caller) checkVolume(tag string, want float64) error {
	var pp struct {
		Volume float64 `json:"volume"`
	}
	c.json("get_physical_properties", nil, &pp)
	if c.err != nil {
		return c.err
	}
	rel := math.Abs(pp.Volume-want) / want
	fmt.Printf("%-18s volume = %.6f cm^3  want ~%.6f  (rel %.4f)\n", tag, pp.Volume, want, rel)
	if rel > 0.02 {
		return fmt.Errorf("%s volume off by %.2f%%", tag, rel*100)
	}
	return nil
}

func firstText(res *mcp.CallToolResult) string {
	for _, ct := range res.Content {
		if tc, ok := ct.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
