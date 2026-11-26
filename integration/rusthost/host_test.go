// SPDX-License-Identifier: GPL-2.0-only

package rusthost

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connectRustHost(t *testing.T, ctx context.Context) *mcp.ClientSession {
	t.Helper()
	host := os.Getenv("OBK_RUST_MCP_HOST")
	if host == "" {
		t.Skip("run make test-rust to start the Rust desktop with the MCP Bridge")
	}
	host, err := filepath.Abs(host)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	addins := filepath.Join(root, "addins")
	stageBridge(t, addins, os.Getenv("OBK_RUST_MCP_LIBRARY"))
	address := startRustHost(t, ctx, host, root, addins)
	client := mcp.NewClient(&mcp.Implementation{Name: "rust-host-integration", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: "http://" + address + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("connect to Rust host MCP server: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func stageBridge(t *testing.T, dir, library string) {
	t.Helper()
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(library)
	if err != nil {
		t.Fatalf("read built MCP Bridge: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.Base(library)), contents, 0o600); err != nil {
		t.Fatalf("stage MCP Bridge: %v", err)
	}
}

func startRustHost(t *testing.T, ctx context.Context, host, root, addins string) string {
	t.Helper()
	log, err := os.Create(filepath.Join(root, "host.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	// The bridge binds an OS-assigned port, so test processes can run independently.
	cmd := exec.CommandContext(ctx, host)
	cmd.Dir = root
	cmd.Env = append(cmd.Environ(),
		"OBK_CONFIG_DIR="+filepath.Join(root, "config"),
		"OBK_ADDINS_DIR="+addins,
		"OBK_MCP_ADDR=127.0.0.1:0",
	)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatalf("start Rust host %s: %v", host, err)
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	t.Cleanup(func() {
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Errorf("stop Rust host: %v", err)
		}
		<-exited
		if t.Failed() {
			output, _ := os.ReadFile(log.Name())
			t.Logf("Rust host output:\n%s", output)
		}
	})
	return waitForBridge(t, ctx, cmd, log.Name(), exited)
}

var bridgeAddress = regexp.MustCompile(`mcp-bridge: serving [0-9]+ tools on (127\.0\.0\.1:[0-9]+)`)

func waitForBridge(t *testing.T, ctx context.Context, cmd *exec.Cmd, logPath string, exited <-chan struct{}) string {
	t.Helper()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		output, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read Rust host log: %v", err)
		}
		if match := bridgeAddress.FindSubmatch(output); match != nil {
			return string(match[1])
		}
		select {
		case <-exited:
			t.Fatalf("Rust host exited before MCP Bridge bound its port: %v", cmd.ProcessState)
		case <-ctx.Done():
			t.Fatalf("wait for MCP Bridge startup: %v", ctx.Err())
		case <-tick.C:
		}
	}
}
