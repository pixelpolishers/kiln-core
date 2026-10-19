package options

import (
	"strings"
	"testing"

	"github.com/pixelpolishers/kiln-core/manifest"
)

var specs = map[string]map[string]manifest.Option{
	"fmtlib/fmt": {
		"header_only": {Default: false},
		"level":       {Default: float64(1), Values: []any{float64(1), float64(2), float64(3)}},
		"mode":        {Default: "fast", Values: []any{"fast", "safe"}},
	},
	"plain/pkg": {},
}

func TestDefaults(t *testing.T) {
	got, err := Resolve(specs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got["fmtlib/fmt"]["header_only"] != false || got["fmtlib/fmt"]["mode"] != "fast" || len(got["plain/pkg"]) != 0 {
		t.Errorf("%v", got)
	}
}

func TestRequests(t *testing.T) {
	got, err := Resolve(specs, []Request{
		{Package: "fmtlib/fmt", Option: "header_only", Value: true, From: "a@1.0.0", Root: false},
		{Package: "fmtlib/fmt", Option: "header_only", Value: true, From: "b@1.0.0", Root: false}, // agreeing dependents are fine
		{Package: "not/selected", Option: "x", Value: 1, From: "a@1.0.0", Root: false},            // ignored
	})
	if err != nil || got["fmtlib/fmt"]["header_only"] != true {
		t.Fatalf("%v %v", got, err)
	}
}

func TestConflictAndRootOverride(t *testing.T) {
	reqs := []Request{
		{Package: "fmtlib/fmt", Option: "header_only", Value: true, From: "a@1.0.0", Root: false},
		{Package: "fmtlib/fmt", Option: "header_only", Value: false, From: "b@2.0.0", Root: false},
	}
	_, err := Resolve(specs, reqs)
	if err == nil || !strings.Contains(err.Error(), "a@1.0.0 wants true, b@2.0.0 wants false") || !strings.Contains(err.Error(), "-o fmtlib/fmt:header_only=VALUE") {
		t.Fatalf("%v", err)
	}
	got, err := Resolve(specs, append(reqs, Request{Package: "fmtlib/fmt", Option: "header_only", Value: true, From: "root", Root: true}))
	if err != nil || got["fmtlib/fmt"]["header_only"] != true {
		t.Fatalf("root must win: %v %v", got, err)
	}
}

func TestValidation(t *testing.T) {
	cases := map[string]Request{
		"no such option":  {Package: "fmtlib/fmt", Option: "nope", Value: true, From: "root", Root: true},
		"wrong type":      {Package: "fmtlib/fmt", Option: "header_only", Value: "yes", From: "root", Root: true},
		"not allowed num": {Package: "fmtlib/fmt", Option: "level", Value: float64(9), From: "root", Root: true},
		"not allowed str": {Package: "fmtlib/fmt", Option: "mode", Value: "slow", From: "root", Root: true},
	}
	for name, r := range cases {
		if _, err := Resolve(specs, []Request{r}); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	if _, err := Resolve(specs, []Request{{Package: "fmtlib/fmt", Option: "mode", Value: "safe", From: "root", Root: true}}); err != nil {
		t.Errorf("valid value rejected: %v", err)
	}
}

func TestRawOverrideIsTypedByTheOption(t *testing.T) {
	got, err := Resolve(specs, []Request{
		{Package: "fmtlib/fmt", Option: "header_only", Raw: "true", From: "root (-o)", Root: true},
		{Package: "fmtlib/fmt", Option: "level", Raw: "3", From: "root (-o)", Root: true},
	})
	if err != nil || got["fmtlib/fmt"]["header_only"] != true || got["fmtlib/fmt"]["level"] != float64(3) {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := Resolve(specs, []Request{{Package: "fmtlib/fmt", Option: "header_only", Raw: "maybe", Root: true}}); err == nil {
		t.Error("a raw value of the wrong type must fail")
	}
	if _, err := Resolve(specs, []Request{{Package: "fmtlib/fmt", Option: "level", Raw: "9", Root: true}}); err == nil {
		t.Error("a raw value outside the allowed list must fail")
	}
}

func TestParse(t *testing.T) {
	if v, err := Parse(specs["fmtlib/fmt"]["header_only"], "true"); err != nil || v != true {
		t.Errorf("%v %v", v, err)
	}
	if v, err := Parse(specs["fmtlib/fmt"]["level"], "2"); err != nil || v != float64(2) {
		t.Errorf("%v %v", v, err)
	}
	if v, err := Parse(specs["fmtlib/fmt"]["mode"], "safe"); err != nil || v != "safe" {
		t.Errorf("%v %v", v, err)
	}
	if _, err := Parse(specs["fmtlib/fmt"]["header_only"], "maybe"); err == nil {
		t.Error("want error")
	}
}
