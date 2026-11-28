// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"encoding/json"
	"testing"
)

func TestInMemoryBridgeToolsAndAutomation(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"materials":[{"id":"mat.steel","displayName":"Steel"}]}`)}
	srv, err := NewServer(host)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ib, err := NewInMemoryBridge(srv)
	if err != nil {
		t.Fatalf("NewInMemoryBridge: %v", err)
	}
	defer func() { _ = ib.Close() }()

	ctx := context.Background()
	tools, err := ib.Tools(ctx)
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}
	if len(tools) == 0 {
		t.Fatal("expected non-empty MCP tool list")
	}

	rawList, err := ib.HandleAutomation(ctx, "mcp.listTools", nil)
	if err != nil {
		t.Fatalf("mcp.listTools: %v", err)
	}
	var listResp struct {
		Tools []ToolDescriptor `json:"tools"`
	}
	if err := json.Unmarshal(rawList, &listResp); err != nil || len(listResp.Tools) != len(tools) {
		t.Fatalf("mcp.listTools decode: len=%d want=%d err=%v", len(listResp.Tools), len(tools), err)
	}

	rawCall, err := ib.HandleAutomation(ctx, "mcp.callTool", []byte(`{"name":"list_materials","arguments":{}}`))
	if err != nil {
		t.Fatalf("mcp.callTool: %v", err)
	}
	if len(rawCall) == 0 || host.lastMethod != "materials.list" {
		t.Fatalf("mcp.callTool: method=%q raw=%q", host.lastMethod, rawCall)
	}

	if _, err := ib.HandleAutomation(ctx, "mcp.unknown", nil); err == nil {
		t.Fatal("expected error for unknown automation method")
	}
}

func TestInMemoryBridgeJSONRPC(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"materials":[{"id":"mat.steel","displayName":"Steel"}]}`)}
	srv, err := NewServer(host)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ib, err := NewInMemoryBridge(srv)
	if err != nil {
		t.Fatalf("NewInMemoryBridge: %v", err)
	}
	defer func() { _ = ib.Close() }()

	ctx := context.Background()
	for _, req := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_materials","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/list"}`,
	} {
		out, err := ib.HandleAutomation(ctx, "mcp.jsonrpc", []byte(req))
		if err != nil {
			t.Fatalf("jsonrpc %s: %v", req, err)
		}
		var env jsonRPCResponse
		if err := json.Unmarshal(out, &env); err != nil {
			t.Fatalf("unmarshal %q: %v", out, err)
		}
		if env.Error != nil {
			t.Fatalf("jsonrpc %s returned error: %+v", req, env.Error)
		}
	}

	// Unknown method returns JSON-RPC error payload.
	errOut, err := ib.HandleAutomation(ctx, "mcp.jsonrpc", []byte(`{"jsonrpc":"2.0","id":9,"method":"no/such"}`))
	if err != nil {
		t.Fatalf("unknown jsonrpc: %v", err)
	}
	var errEnv jsonRPCResponse
	if err := json.Unmarshal(errOut, &errEnv); err != nil || errEnv.Error == nil {
		t.Fatalf("expected JSON-RPC error object, got %q", errOut)
	}
}
