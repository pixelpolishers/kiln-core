package when

import "testing"

func TestMatch(t *testing.T) {
	ctx := Context(
		map[string]any{"os": "linux", "arch": "x86_64", "cpp_standard": "17", "compiler": map[string]any{"name": "gcc", "version": "13"}},
		map[string]any{"header_only": false, "level": 3},
	)
	cases := []struct {
		cond map[string]any
		want bool
	}{
		{nil, true},
		{map[string]any{}, true},
		{map[string]any{"os": "linux"}, true},
		{map[string]any{"os": "windows"}, false},
		{map[string]any{"os": []any{"windows", "linux"}}, true},
		{map[string]any{"os": []any{"windows", "macos"}}, false},
		{map[string]any{"os": "linux", "arch": "arm64"}, false}, // AND
		{map[string]any{"os": "linux", "arch": "x86_64"}, true},
		{map[string]any{"compiler.name": "gcc"}, true},
		{map[string]any{"compiler.version": "13"}, true},
		{map[string]any{"cpp_standard": float64(17)}, true}, // JSON numbers decode as float64
		{map[string]any{"options.header_only": false}, true},
		{map[string]any{"options.header_only": true}, false},
		{map[string]any{"options.level": float64(3)}, true},
		{map[string]any{"msvc_runtime": "dynamic"}, false}, // absent key never matches
		{map[string]any{"options.missing": false}, false},
	}
	for i, c := range cases {
		if got := Match(c.cond, ctx); got != c.want {
			t.Errorf("case %d %v: got %v, want %v", i, c.cond, got, c.want)
		}
	}
}
