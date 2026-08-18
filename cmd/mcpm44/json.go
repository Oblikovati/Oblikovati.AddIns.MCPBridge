// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// jsonGet walks a dotted path into a JSON object and returns the leaf value.
func jsonGet(txt, path string) any {
	var m map[string]any
	if json.Unmarshal([]byte(txt), &m) != nil {
		return nil
	}
	var cur any = m
	for _, key := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = obj[key]
	}
	return cur
}

func jsonStr(txt, path string) string {
	if s, ok := jsonGet(txt, path).(string); ok {
		return s
	}
	return ""
}

func jsonInt(txt, path string) int {
	if f, ok := jsonGet(txt, path).(float64); ok {
		return int(f)
	}
	return 0
}

func jsonUint(txt, path string) uint64 {
	if f, ok := jsonGet(txt, path).(float64); ok {
		return uint64(f)
	}
	return 0
}

// firstPartID returns the id of the first part document in a list_documents payload.
func firstPartID(txt string) uint64 {
	var r struct {
		Documents []struct {
			ID   uint64 `json:"id"`
			Type string `json:"type"`
		} `json:"documents"`
	}
	if json.Unmarshal([]byte(txt), &r) != nil {
		return 0
	}
	for _, doc := range r.Documents {
		if doc.Type == "part" {
			return doc.ID
		}
	}
	return 0
}

// firstPartName returns the name of the first part document in a list_documents payload.
func firstPartName(txt string) string {
	var r struct {
		Documents []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"documents"`
	}
	if json.Unmarshal([]byte(txt), &r) != nil {
		return ""
	}
	for _, doc := range r.Documents {
		if doc.Type == "part" {
			return doc.Name
		}
	}
	return ""
}

// docIDByName returns the id of the open document with the given name.
func docIDByName(txt, name string) uint64 {
	var r struct {
		Documents []struct {
			ID   uint64 `json:"id"`
			Name string `json:"name"`
		} `json:"documents"`
	}
	if json.Unmarshal([]byte(txt), &r) != nil {
		return 0
	}
	for _, doc := range r.Documents {
		if doc.Name == name {
			return doc.ID
		}
	}
	return 0
}

func assertJSONBool(dr *d, txt, path string, want bool, label string) {
	got, ok := jsonGet(txt, path).(bool)
	if !ok || got != want {
		dr.fail++
		fmt.Printf("  WARN %-28s %s = %v, want %v\n", "assert", label, jsonGet(txt, path), want)
		return
	}
	fmt.Printf("  OK   %-28s %s\n", "assert", label)
}

func assertEq(dr *d, got, want int, label string) {
	if got != want {
		dr.fail++
		fmt.Printf("  WARN %-28s %s = %d, want %d\n", "assert", label, got, want)
		return
	}
	fmt.Printf("  OK   %-28s %s = %d\n", "assert", label, got)
}

func assertContains(dr *d, txt, sub, label string) {
	if !strings.Contains(txt, sub) {
		dr.fail++
		fmt.Printf("  WARN %-28s %s (missing %q)\n", "assert", label, sub)
		return
	}
	fmt.Printf("  OK   %-28s %s\n", "assert", label)
}
