// SPDX-License-Identifier: GPL-2.0-only

// Command oblikovati-mcp-bridge is built as a c-shared library (.so/.dll) and loaded
// by the Oblikovati host at runtime. It implements the C ABI in oblikovati_addin.h
// (vendored from the oblikovati.org/api module into ./include by `make sync-header`):
// on Activate it starts an MCP server (HTTP/SSE,
// package bridge) that forwards LLM tool calls back to the host through the
// host-supplied callback. The host owns the model; this library owns only the MCP
// server (running in its own Go runtime — see the package memory for the two-runtime
// rationale).
package main

/*
#cgo CFLAGS: -I${SRCDIR}/include -DOBK_BUILDING_ADDIN
#include <stdlib.h>
#include <stdint.h>
#include "oblikovati_addin.h"
*/
import "C"
import (
	"os"
	"sync"
	"unsafe"

	"oblikovati.org/mcp-bridge/bridge"
)

const addInID = "com.oblikovati.mcp-bridge"

// defaultAddr is the loopback MCP endpoint; override with OBK_MCP_ADDR.
const defaultAddr = "127.0.0.1:7800"

var (
	idC  = C.CString(addInID)
	manC = C.CString(manifestJSON)

	mu       sync.Mutex     // guards the host callbacks and the server
	hostCall C.ObkHostCall  // host RPC entry (set on Activate)
	hostFree C.ObkHostFree  // frees host-owned response buffers
	server   *bridge.Server // running MCP server, nil when inactive
)

//export ObkAddInId
func ObkAddInId() *C.char { return idC }

//export ObkAddInManifest
func ObkAddInManifest() *C.char { return manC }

//export ObkAddInActivate
func ObkAddInActivate(call C.ObkHostCall, freeFn C.ObkHostFree) C.int {
	mu.Lock()
	defer mu.Unlock()
	if server != nil { // idempotent
		return C.OBK_OK
	}
	hostCall, hostFree = call, freeFn
	srv, err := bridge.NewServer(cgoHostCaller{})
	if err != nil {
		return C.OBK_ERR
	}
	if err := srv.Start(addr()); err != nil {
		return C.OBK_ERR
	}
	server = srv
	return C.OBK_OK
}

//export ObkAddInDeactivate
func ObkAddInDeactivate() C.int {
	mu.Lock()
	defer mu.Unlock()
	if server == nil {
		return C.OBK_OK
	}
	_ = server.Stop()
	server = nil
	hostCall, hostFree = nil, nil
	return C.OBK_OK
}

//export ObkAddInNotify
func ObkAddInNotify(ev *C.uint8_t, n C.int) C.int {
	mu.Lock()
	srv := server
	mu.Unlock()
	if srv == nil {
		return C.OBK_OK
	}
	srv.Notify(C.GoBytes(unsafe.Pointer(ev), n))
	return C.OBK_OK
}

//export ObkFree
func ObkFree(p *C.uint8_t) { C.free(unsafe.Pointer(p)) }

// addr returns the MCP listen address (OBK_MCP_ADDR or the loopback default).
func addr() string {
	if a := os.Getenv("OBK_MCP_ADDR"); a != "" {
		return a
	}
	return defaultAddr
}

func main() {}
