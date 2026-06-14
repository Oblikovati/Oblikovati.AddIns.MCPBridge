// SPDX-License-Identifier: GPL-2.0-only

// Command mcpcamerastress is an integration performance test for the camera API: it
// orbits the viewport camera at a target frame rate (default 60 Hz) against a running
// oblikovati-mcp-bridge endpoint and measures how well the API keeps up. This is the
// presenter-follow workload from the oblikovati-meeting ADR-0003 (camera streamed at
// 30–60 Hz) exercised end to end: MCP client → bridge set_camera → host router
// view.setCamera → live scene.Camera.
//
// It reports the client-observed round-trip latency (includes HTTP/MCP/C-ABI overhead),
// the host-side view.setCamera processing time (from the operation trace), the achieved
// rate, and how many frames blew the per-frame budget — enough to decide whether 60 Hz is
// sustainable or the stream must be throttled. Exits non-zero on any transport error or a
// caught host panic.
//
// Usage: mcpcamerastress [--url http://127.0.0.1:7800/mcp] [--hz 60] [--duration 5s] [--radius 50]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
)

type cameraView struct {
	Eye    [3]float64 `json:"eye"`
	Target [3]float64 `json:"target"`
	Up     [3]float64 `json:"up"`
	FOV    float64    `json:"fov"`
}

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint URL")
	hz := flag.Float64("hz", 60, "target camera update rate (Hz)")
	duration := flag.Duration("duration", 5*time.Second, "how long to drive the camera")
	radius := flag.Float64("radius", 50, "orbit radius around the target (model units)")
	flag.Parse()
	if err := run(*url, *hz, *duration, *radius); err != nil {
		fmt.Fprintln(os.Stderr, "mcpcamerastress:", err)
		os.Exit(1)
	}
}

func run(url string, hz float64, duration time.Duration, radius float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), duration+30*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "mcpcamerastress", Version: "0.1.0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer cs.Close()

	// Seed the orbit from the live camera so target/up/fov match the running view.
	base, err := getCamera(ctx, cs)
	if err != nil {
		return err
	}
	budget := time.Duration(float64(time.Second) / hz)
	fmt.Printf("camera stress: %.0f Hz for %s (budget %s/frame), orbit r=%.1f @ %s\n\n",
		hz, duration, budget.Round(time.Microsecond), radius, url)

	traceCursor := drainCursor(ctx, cs) // ignore pre-existing trace records
	lat := make([]time.Duration, 0, int(hz*duration.Seconds())+1)
	var overruns, transportErr, hostErr int

	ticker := time.NewTicker(budget)
	defer ticker.Stop()
	deadline := time.Now().Add(duration)
	frame := 0
	for now := range ticker.C {
		if now.After(deadline) {
			break
		}
		eye := orbit(base.Target, radius, float64(frame)/hz)
		start := time.Now()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "set_camera", Arguments: map[string]any{
			"eye": eye, "target": base.Target, "up": base.Up, "fov": base.FOV,
		}})
		elapsed := time.Since(start)
		switch {
		case err != nil:
			transportErr++
		case res.IsError:
			hostErr++
		default:
			lat = append(lat, elapsed)
			if elapsed > budget {
				overruns++
			}
		}
		frame++
	}

	wall := lat
	reportClient(hz, budget, duration, wall, overruns, transportErr, hostErr)
	if err := reportHost(ctx, cs, traceCursor); err != nil {
		return err
	}
	if transportErr > 0 {
		return fmt.Errorf("%d transport error(s) — bridge/endpoint problem", transportErr)
	}
	return nil
}

// orbit returns an eye position circling target at radius, advancing with time t (seconds),
// with a gentle vertical bob so the motion sweeps in all three axes.
func orbit(target [3]float64, radius, t float64) [3]float64 {
	const turnsPerSec = 0.5 // half a revolution per second — fast but smooth
	a := 2 * math.Pi * turnsPerSec * t
	return [3]float64{
		target[0] + radius*math.Cos(a),
		target[1] + radius*0.4*math.Sin(2*a),
		target[2] + radius*math.Sin(a),
	}
}

func reportClient(hz float64, budget, duration time.Duration, lat []time.Duration, overruns, transportErr, hostErr int) {
	fmt.Printf("client round-trip (MCP→bridge→C-ABI→host):\n")
	if len(lat) == 0 {
		fmt.Printf("  no successful frames (transport=%d host=%d)\n", transportErr, hostErr)
		return
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	var total time.Duration
	for _, d := range lat {
		total += d
	}
	achieved := float64(len(lat)) / duration.Seconds()
	fmt.Printf("  frames=%d transport_err=%d host_err=%d\n", len(lat), transportErr, hostErr)
	fmt.Printf("  latency min=%s p50=%s p95=%s p99=%s max=%s avg=%s\n",
		r(lat[0]), r(pct(lat, 50)), r(pct(lat, 95)), r(pct(lat, 99)), r(lat[len(lat)-1]), r(total/time.Duration(len(lat))))
	fmt.Printf("  achieved %.1f Hz of %.0f Hz target; %d/%d frames over budget (%s)\n",
		achieved, hz, overruns, len(lat), budget.Round(time.Microsecond))
	verdict := "SUSTAINS target rate"
	if pct(lat, 95) > budget || achieved < hz*0.95 {
		verdict = "DOES NOT sustain target rate (latency-bound — throttle the stream, per ADR-0003)"
	}
	fmt.Printf("  verdict: %s\n", verdict)
}

// reportHost reads the operation trace for the host-side view.setCamera processing time
// (excludes network/C-ABI), the truest measure of the API's own cost.
func reportHost(ctx context.Context, cs *mcp.ClientSession, since uint64) error {
	var out wire.LogsResult
	if !callJSON(ctx, cs, "tail_logs", map[string]any{"sinceSeq": since, "max": 1000000}, &out) {
		fmt.Println("\nhost trace: unavailable")
		return nil
	}
	host := make([]time.Duration, 0, len(out.Records))
	var errs, panics int
	for _, r := range out.Records {
		if r.Panic != "" {
			panics++
			fmt.Printf("\nPANIC in %s: %s\n%s\n", r.Method, r.Panic, r.Stack)
			continue
		}
		if r.Method == wire.MethodViewSetCamera {
			host = append(host, time.Duration(r.DurationMicros)*time.Microsecond)
			if !r.OK {
				errs++
			}
		}
	}
	fmt.Printf("\nhost-side view.setCamera (router processing only):\n")
	if len(host) == 0 {
		fmt.Println("  no trace samples")
	} else {
		sort.Slice(host, func(i, j int) bool { return host[i] < host[j] })
		fmt.Printf("  n=%d err=%d  p50=%s p95=%s max=%s\n", len(host), errs,
			r(pct(host, 50)), r(pct(host, 95)), r(host[len(host)-1]))
	}
	if panics > 0 {
		return fmt.Errorf("%d host panic(s) caught during the stress run", panics)
	}
	return nil
}

func getCamera(ctx context.Context, cs *mcp.ClientSession) (cameraView, error) {
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_camera"})
	if err != nil {
		return cameraView{}, fmt.Errorf("get_camera: %w", err)
	}
	var cv cameraView
	if err := json.Unmarshal([]byte(firstText(res)), &cv); err != nil {
		return cv, fmt.Errorf("decode camera %q: %w", firstText(res), err)
	}
	return cv, nil
}

// drainCursor returns the current trace tail cursor so the run only measures its own frames.
func drainCursor(ctx context.Context, cs *mcp.ClientSession) uint64 {
	var out wire.LogsResult
	if callJSON(ctx, cs, "tail_logs", map[string]any{"sinceSeq": uint64(0), "max": 1000000}, &out) {
		return out.NextSeq
	}
	return 0
}

func callJSON(ctx context.Context, cs *mcp.ClientSession, tool string, args map[string]any, v any) bool {
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil || res.IsError {
		return false
	}
	return json.Unmarshal([]byte(firstText(res)), v) == nil
}

func firstText(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func pct(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := (len(sorted) * p) / 100
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return sorted[i]
}

func r(d time.Duration) time.Duration { return d.Round(time.Microsecond) }
