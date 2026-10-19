// Package when evaluates the "when" conditions used in manifests.
//
// A condition maps keys to a value or a list of values. All keys must match
// (AND); a list matches if any entry equals the actual value (OR). Keys are
// settings paths ("os", "compiler.name", "build_type", ...) or
// "options.<name>".
package when

import (
	"fmt"
	"strings"
)

// Context flattens a settings object and resolved options into the keys a
// condition can refer to ("compiler.name", "options.header_only", ...).
func Context(settings, options map[string]any) map[string]string {
	ctx := map[string]string{}
	var walk func(prefix string, m map[string]any)
	walk = func(prefix string, m map[string]any) {
		for k, v := range m {
			if sub, ok := v.(map[string]any); ok {
				walk(prefix+k+".", sub)
				continue
			}
			ctx[prefix+k] = fmt.Sprint(v)
		}
	}
	walk("", settings)
	for k, v := range options {
		ctx["options."+k] = fmt.Sprint(v)
	}
	return ctx
}

// Match reports whether cond holds in ctx. A nil or empty condition always holds.
// A key absent from ctx never matches.
func Match(cond map[string]any, ctx map[string]string) bool {
	for key, want := range cond {
		got, ok := ctx[key]
		if !ok {
			return false
		}
		if !matchValue(want, got) {
			return false
		}
	}
	return true
}

func matchValue(want any, got string) bool {
	if list, ok := want.([]any); ok {
		for _, w := range list {
			if fmt.Sprint(w) == got {
				return true
			}
		}
		return false
	}
	return fmt.Sprint(want) == got
}

// Keys returns the keys of cond, for error messages.
func Keys(cond map[string]any) string {
	ks := make([]string, 0, len(cond))
	for k := range cond {
		ks = append(ks, k)
	}
	return strings.Join(ks, ", ")
}
