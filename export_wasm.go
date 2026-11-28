//go:build js && wasm

// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"syscall/js"

	"oblikovati.org/api/client"
	"oblikovati.org/api/wasmaddin"
	"oblikovati.org/mcp-bridge/bridge"
)

const addInID = "com.oblikovati.mcp-bridge"

type wasmMCPAddIn struct {
	mu       sync.Mutex
	server   *bridge.Server
	inMemory *bridge.InMemoryBridge
	relayWS  js.Value
}

func (a *wasmMCPAddIn) ID() string       { return addInID }
func (a *wasmMCPAddIn) Manifest() string { return manifestJSON }

func (a *wasmMCPAddIn) Activate(caller client.Caller) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	srv, err := bridge.NewServer(caller)
	if err != nil {
		return err
	}
	ib, err := bridge.NewInMemoryBridge(srv)
	if err != nil {
		return err
	}
	a.server = srv
	a.inMemory = ib
	a.wireBrowserChannels()
	go a.publishWebMCPTools(ib, srv.ToolCount())
	return nil
}

func (a *wasmMCPAddIn) Deactivate() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closeRelayLocked()
	var err error
	if a.inMemory != nil {
		err = a.inMemory.Close()
		a.inMemory = nil
	}
	a.server = nil
	return err
}

func (a *wasmMCPAddIn) Notify(event []byte) {
	a.mu.Lock()
	srv := a.server
	a.mu.Unlock()
	if srv != nil {
		srv.Notify(event)
	}
}

func (a *wasmMCPAddIn) Automation(method string, req []byte) ([]byte, error) {
	a.mu.Lock()
	ib := a.inMemory
	a.mu.Unlock()
	if ib == nil {
		return nil, fmt.Errorf("mcp-bridge: add-in is not active")
	}
	return ib.HandleAutomation(context.Background(), method, req)
}

func (a *wasmMCPAddIn) publishWebMCPTools(ib *bridge.InMemoryBridge, count int) {
	tools, err := ib.Tools(context.Background())
	if err != nil {
		logWasm(fmt.Sprintf("mcp-bridge: failed to list tools for WebMCP: %v", err))
		return
	}
	raw, err := json.Marshal(map[string]any{"tools": tools})
	if err != nil {
		return
	}
	payload := js.Global().Get("JSON").Call("parse", string(raw))
	host := wasmaddin.Host()
	host.Call("emit", "webmcp:register", payload)
	logWasm(fmt.Sprintf("mcp-bridge: registered %d WebMCP tools", count))
}

func (a *wasmMCPAddIn) wireBrowserChannels() {
	host := wasmaddin.Host()
	if host.IsUndefined() || host.Get("onChannel").IsUndefined() {
		return
	}
	host.Call("onChannel", "mcp:relay:connect", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 && !args[0].IsUndefined() && !args[0].IsNull() {
			url := args[0].Get("url").String()
			if url != "" {
				a.connectRelay(url)
			}
		}
		return nil
	}))
	host.Call("onChannel", "mcp:relay:disconnect", js.FuncOf(func(js.Value, []js.Value) any {
		a.mu.Lock()
		a.closeRelayLocked()
		a.mu.Unlock()
		return nil
	}))
}

func (a *wasmMCPAddIn) connectRelay(url string) {
	wsCtor := js.Global().Get("WebSocket")
	if wsCtor.IsUndefined() {
		logWasm("mcp-bridge: WebSocket is unavailable in this environment")
		return
	}
	a.mu.Lock()
	a.closeRelayLocked()
	ws := wsCtor.New(url)
	a.relayWS = ws
	a.mu.Unlock()

	ws.Set("onopen", js.FuncOf(func(js.Value, []js.Value) any {
		logWasm("mcp-bridge: connected to WebSocket MCP relay " + url)
		return nil
	}))
	ws.Set("onmessage", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		frame := args[0].Get("data").String()
		go a.handleRelayFrame(ws, frame)
		return nil
	}))
	ws.Set("onerror", js.FuncOf(func(js.Value, []js.Value) any {
		logWasm("mcp-bridge: WebSocket MCP relay error on " + url)
		return nil
	}))
}

func (a *wasmMCPAddIn) handleRelayFrame(ws js.Value, frame string) {
	reply, err := a.Automation("mcp.jsonrpc", []byte(frame))
	if err != nil || len(reply) == 0 {
		return
	}
	if ws.Get("readyState").Int() == 1 {
		ws.Call("send", string(reply))
	}
}

func (a *wasmMCPAddIn) closeRelayLocked() {
	if !a.relayWS.IsUndefined() && !a.relayWS.IsNull() {
		a.relayWS.Call("close")
		a.relayWS = js.Undefined()
	}
}

func logWasm(msg string) {
	host := wasmaddin.Host()
	if !host.IsUndefined() && !host.Get("log").IsUndefined() {
		host.Call("log", msg)
	}
}

func main() {
	wasmaddin.Serve(&wasmMCPAddIn{})
}
