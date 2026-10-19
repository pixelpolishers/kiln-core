// Package options resolves package options across a dependency graph.
//
// Every package declares options with defaults. Dependents may set options of
// their dependencies, and the root project can override anything. Two
// dependents that ask for different values of the same option conflict unless
// the root decides.
package options

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/pixelpolishers/kiln-core/manifest"
)

// Request asks for an option value.
type Request struct {
	Package string
	Option  string
	Value   any
	// Raw is the text of a command-line override (-o pkg:opt=value). It is
	// converted to the option's declared type, replacing Value.
	Raw  string
	From string // who asked, for error messages ("root" or "name@version")
	Root bool   // root requests override everything
}

// Resolve returns, for every package, the value of each option it declares.
// specs maps package name to its declared options.
func Resolve(specs map[string]map[string]manifest.Option, reqs []Request) (map[string]map[string]any, error) {
	out := map[string]map[string]any{}
	for pkg, opts := range specs {
		out[pkg] = map[string]any{}
		for name, o := range opts {
			out[pkg][name] = o.Default
		}
	}

	type key struct{ pkg, opt string }
	grouped := map[key][]Request{}
	var order []key
	for _, r := range reqs {
		spec, known := specs[r.Package]
		if !known {
			continue // not part of the solution
		}
		o, ok := spec[r.Option]
		if !ok {
			return nil, fmt.Errorf("%s sets option %q, but %s has no such option (it declares: %s)", r.From, r.Option, r.Package, declared(spec))
		}
		if r.Raw != "" {
			v, err := Parse(o, r.Raw)
			if err != nil {
				return nil, fmt.Errorf("%s sets %s:%s: %w", r.From, r.Package, r.Option, err)
			}
			r.Value = v
		}
		if err := check(o, r.Value); err != nil {
			return nil, fmt.Errorf("%s sets %s:%s: %w", r.From, r.Package, r.Option, err)
		}
		k := key{r.Package, r.Option}
		if _, seen := grouped[k]; !seen {
			order = append(order, k)
		}
		grouped[k] = append(grouped[k], r)
	}

	for _, k := range order {
		rs := grouped[k]
		var chosen *Request
		for i := range rs {
			if rs[i].Root {
				chosen = &rs[i] // the last root request wins
			}
		}
		if chosen == nil {
			chosen = &rs[0]
			for _, r := range rs[1:] {
				if fmt.Sprint(r.Value) != fmt.Sprint(chosen.Value) {
					return nil, fmt.Errorf("conflicting values for option %s of %s: %s wants %v, %s wants %v; set it explicitly with -o %s:%s=VALUE",
						k.opt, k.pkg, chosen.From, chosen.Value, r.From, r.Value, k.pkg, k.opt)
				}
			}
		}
		out[k.pkg][k.opt] = chosen.Value
	}
	return out, nil
}

func declared(spec map[string]manifest.Option) string {
	if len(spec) == 0 {
		return "none"
	}
	names := make([]string, 0, len(spec))
	for n := range spec {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func kind(v any) string {
	switch v.(type) {
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64, float32, int, int64:
		return "number"
	}
	return fmt.Sprintf("%T", v)
}

// check validates a value against an option's type and allowed values.
func check(o manifest.Option, v any) error {
	if o.Default != nil && kind(v) != kind(o.Default) {
		return fmt.Errorf("expected a %s, got %v", kind(o.Default), v)
	}
	if len(o.Values) > 0 {
		for _, allowed := range o.Values {
			if fmt.Sprint(allowed) == fmt.Sprint(v) {
				return nil
			}
		}
		allowed := make([]string, len(o.Values))
		for i, a := range o.Values {
			allowed[i] = fmt.Sprint(a)
		}
		return fmt.Errorf("%v is not one of: %s", v, strings.Join(allowed, ", "))
	}
	return nil
}

// Parse converts the text of an -o flag into a value of the option's type.
func Parse(o manifest.Option, raw string) (any, error) {
	switch kind(o.Default) {
	case "boolean":
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("%q is not a boolean", raw)
		}
		return b, nil
	case "number":
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", raw)
		}
		return n, nil
	}
	return raw, nil
}
