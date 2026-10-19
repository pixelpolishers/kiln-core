// Package assetid computes the identity of a built binary.
//
// Two binaries are interchangeable only if everything that can change the
// compiled output matches: the package and version, the build settings that
// matter for that package, its options, and the exact builds of its
// dependencies. The id is the SHA-256 of the canonical JSON of those facts.
// The client computes it locally and looks it up in the provider's asset
// manifest: there is no "close enough" matching.
package assetid

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Dep identifies one dependency build.
type Dep struct {
	Version string
	ID      string
}

// Input is everything the id is derived from.
type Input struct {
	Package string
	Version string
	// Settings must already be reduced with Relevant.
	Settings map[string]any
	Options  map[string]any
	Deps     map[string]Dep
}

// Compute returns the asset id for in.
func Compute(in Input) (string, error) {
	deps := map[string]any{}
	for name, d := range in.Deps {
		deps[name] = map[string]any{"version": d.Version, "id": d.ID}
	}
	doc := map[string]any{
		"package":      in.Package,
		"version":      in.Version,
		"settings":     nonNil(in.Settings),
		"options":      nonNil(in.Options),
		"dependencies": deps,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf) // map keys are sorted, so the encoding is canonical
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return "", fmt.Errorf("asset id: %w", err)
	}
	sum := sha256.Sum256(bytes.TrimRight(buf.Bytes(), "\n"))
	return hex.EncodeToString(sum[:]), nil
}

func nonNil(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

// Kind describes how much of the build settings a package's binary depends on.
type Kind struct {
	// Language is "c" or "c++" (the manifest's "language", default "c++").
	Language string
	// HeaderOnly is true when no target produces a binary.
	HeaderOnly bool
}

// cxxOnly are the settings a pure C library does not depend on.
var cxxOnly = []string{"cpp_standard", "stdlib", "cxx11_abi", "rtti", "exceptions"}

// Relevant reduces a full settings object to the part that affects a package's
// binary: a header-only package has no binary and so no settings; a C package
// ignores the C++-specific ones. The input is not modified.
func Relevant(settings map[string]any, k Kind) map[string]any {
	if k.HeaderOnly {
		return map[string]any{}
	}
	out := make(map[string]any, len(settings))
	for key, v := range settings {
		out[key] = v
	}
	if k.Language == "c" {
		for _, key := range cxxOnly {
			delete(out, key)
		}
	}
	return out
}
