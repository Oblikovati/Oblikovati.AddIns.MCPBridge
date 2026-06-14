// SPDX-License-Identifier: GPL-2.0-only

// Command mcpnormals drives the LIVE app's debug renders to investigate mesh defects on
// the active document: it sets the camera, captures shaded / normal-debug (green=outward,
// red=back-facing) / per-triangle-color frames, and resets the debug state. The capture
// trio is the fastest way to SEE winding, crack, and sliver problems (used to diagnose
// Oblikovati/Oblikovati#137).
//
// Usage: mcpnormals [--url http://127.0.0.1:7800/mcp] [--eye x,y,z --target x,y,z] [--prefix /tmp/clip]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint URL")
	eye := flag.String("eye", "", "camera eye x,y,z")
	target := flag.String("target", "0,0,0", "camera target x,y,z")
	prefix := flag.String("prefix", "/tmp/clip", "output PNG prefix")
	flag.Parse()
	if err := run(*url, *eye, *target, *prefix); err != nil {
		fmt.Fprintln(os.Stderr, "mcpnormals:", err)
		os.Exit(1)
	}
}

func parseVec(s string) []float64 {
	var x, y, z float64
	fmt.Sscanf(s, "%f,%f,%f", &x, &y, &z)
	return []float64{x, y, z}
}

func run(url, eye, target, prefix string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "mcpnormals", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return err
	}
	defer cs.Close()

	call := func(name string, args map[string]any) error {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if res.IsError {
			for _, c := range res.Content {
				if t, ok := c.(*mcp.TextContent); ok {
					return fmt.Errorf("%s: %s", name, t.Text)
				}
			}
			return fmt.Errorf("%s: tool error", name)
		}
		return nil
	}

	if eye != "" {
		if err := call("set_camera", map[string]any{"eye": parseVec(eye), "target": parseVec(target), "up": []float64{0, 0, 1}, "fov": 0.7}); err != nil {
			return err
		}
	}
	shoot := func(tag string) error {
		if err := call("capture_viewport", map[string]any{"path": prefix + "-" + tag + ".png"}); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond) // the host writes the PNG within a frame
		fmt.Println("wrote", prefix+"-"+tag+".png")
		return nil
	}
	if err := shoot("shaded"); err != nil {
		return err
	}
	if err := call("set_normal_debug", map[string]any{"on": true}); err != nil {
		return err
	}
	if err := shoot("normals"); err != nil {
		return err
	}
	if err := call("set_normal_debug", map[string]any{"on": false}); err != nil {
		return err
	}
	if err := call("set_mesh_colors", map[string]any{"on": true, "perTriangle": true}); err != nil {
		return err
	}
	if err := shoot("triangles"); err != nil {
		return err
	}
	return call("set_mesh_colors", map[string]any{"on": false})
}
