// SPDX-License-Identifier: GPL-2.0-only

// Command mcpcamera live-tests the camera API against a running oblikovati-mcp-bridge
// endpoint: read the camera, move it to a distinct look-at frame, read it back, and
// confirm the change round-tripped through the live host. Also probes a rejected frame.
//
// Usage: mcpcamera [--url http://127.0.0.1:7800/mcp]
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

type cameraView struct {
	Eye    [3]float64 `json:"eye"`
	Target [3]float64 `json:"target"`
	Up     [3]float64 `json:"up"`
	FOV    float64    `json:"fov"`
}

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint URL")
	flag.Parse()
	if err := run(*url); err != nil {
		fmt.Fprintln(os.Stderr, "mcpcamera:", err)
		os.Exit(1)
	}
}

func run(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "mcpcamera", Version: "0.1.0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer cs.Close()

	before, err := getCamera(ctx, cs)
	if err != nil {
		return err
	}
	fmt.Printf("before set_camera: %+v\n", before)

	want := cameraView{Eye: [3]float64{42, 17, 99}, Target: [3]float64{1, 2, 3}, Up: [3]float64{0, 1, 0}, FOV: 0.6}
	set, err := setCamera(ctx, cs, want)
	if err != nil {
		return err
	}
	fmt.Printf("set_camera returned: %+v\n", set)

	after, err := getCamera(ctx, cs)
	if err != nil {
		return err
	}
	fmt.Printf("after  set_camera: %+v\n", after)

	if after.Eye != want.Eye || after.Target != want.Target || after.FOV != want.FOV {
		return fmt.Errorf("FAIL: live camera did not round-trip; want eye=%v target=%v fov=%v, got %+v",
			want.Eye, want.Target, want.FOV, after)
	}
	fmt.Println("PASS: camera round-tripped through the live host")

	// Negative: a degenerate frame (eye == target) must be rejected by the host.
	if _, err := setCamera(ctx, cs, cameraView{Eye: [3]float64{5, 5, 5}, Target: [3]float64{5, 5, 5}, Up: [3]float64{0, 1, 0}, FOV: 0.6}); err != nil {
		fmt.Println("PASS: host rejected degenerate eye==target frame:", err)
	} else {
		return fmt.Errorf("FAIL: host accepted a degenerate eye==target frame")
	}
	return nil
}

func getCamera(ctx context.Context, cs *mcp.ClientSession) (cameraView, error) {
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_camera"})
	if err != nil {
		return cameraView{}, fmt.Errorf("get_camera: %w", err)
	}
	return decodeCamera(res)
}

func setCamera(ctx context.Context, cs *mcp.ClientSession, c cameraView) (cameraView, error) {
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "set_camera", Arguments: map[string]any{
		"eye": c.Eye, "target": c.Target, "up": c.Up, "fov": c.FOV,
	}})
	if err != nil {
		return cameraView{}, fmt.Errorf("set_camera: %w", err)
	}
	if res.IsError {
		return cameraView{}, fmt.Errorf("set_camera rejected: %s", firstText(res))
	}
	return decodeCamera(res)
}

func decodeCamera(res *mcp.CallToolResult) (cameraView, error) {
	var cv cameraView
	if err := json.Unmarshal([]byte(firstText(res)), &cv); err != nil {
		return cv, fmt.Errorf("decode camera %q: %w", firstText(res), err)
	}
	return cv, nil
}

func firstText(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
