package solver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pixelpolishers/kiln-core/constraint"
	"github.com/pixelpolishers/kiln-core/semver"
)

// fake is an in-memory registry: name -> version -> dependencies ("pkg": "constraint").
type fake map[string]map[string]map[string]string

func (f fake) Versions(_ context.Context, name string) ([]semver.Version, error) {
	vs, ok := f[name]
	if !ok {
		return nil, fmt.Errorf("%s: %w", name, ErrUnknownPackage)
	}
	var out []semver.Version
	for v := range vs {
		p, err := semver.Parse(v)
		if err != nil {
			panic(err)
		}
		out = append(out, p) // map order: the solver must not depend on it
	}
	return out, nil
}

func (f fake) Dependencies(_ context.Context, name string, v semver.Version) ([]Dep, error) {
	var out []Dep
	for dep, c := range f[name][v.String()] {
		con, err := constraint.Parse(c)
		if err != nil {
			panic(err)
		}
		out = append(out, Dep{dep, con})
	}
	return out, nil
}

func roots(t *testing.T, kv ...string) []Dep {
	t.Helper()
	var out []Dep
	for i := 0; i < len(kv); i += 2 {
		c, err := constraint.Parse(kv[i+1])
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, Dep{kv[i], c})
	}
	return out
}

func versions(sol Solution) string {
	var parts []string
	for _, p := range sol.Packages {
		parts = append(parts, p.Name+"@"+p.Version.String())
	}
	return strings.Join(parts, " ")
}

func solve(t *testing.T, src Source, kv ...string) (Solution, error) {
	t.Helper()
	return Solve(context.Background(), src, roots(t, kv...), Options{})
}

func TestPicksNewestMatching(t *testing.T) {
	src := fake{"fmt": {"10.2.1": nil, "11.0.1": nil, "11.0.2": nil, "12.0.0": nil}}
	sol, err := solve(t, src, "fmt", "^11.0")
	if err != nil || versions(sol) != "fmt@11.0.2" {
		t.Fatalf("%v %v", versions(sol), err)
	}
}

func TestTransitiveAndOrder(t *testing.T) {
	src := fake{
		"app":    {"1.0.0": {"spdlog": "^1.14", "zlib": "^1.3"}},
		"spdlog": {"1.14.1": {"fmt": "^11.0"}},
		"fmt":    {"11.0.2": nil},
		"zlib":   {"1.3.1": nil},
	}
	sol, err := solve(t, src, "app", "*")
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, p := range sol.Packages {
		pos[p.Name] = i
	}
	if len(pos) != 4 || pos["fmt"] > pos["spdlog"] || pos["spdlog"] > pos["app"] || pos["zlib"] > pos["app"] {
		t.Errorf("dependencies must come first: %s", versions(sol))
	}
	if p, _ := sol.Get("spdlog"); len(p.Requires) != 1 || p.Requires[0] != "fmt" {
		t.Errorf("requires: %+v", p)
	}
}

func TestDiamondSharesOneVersion(t *testing.T) {
	src := fake{
		"a": {"1.0.0": {"c": ">=1.0 <3.0"}},
		"b": {"1.0.0": {"c": "^2.0"}},
		"c": {"1.0.0": nil, "2.0.0": nil, "2.5.0": nil, "3.0.0": nil},
	}
	sol, err := solve(t, src, "a", "*", "b", "*")
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := sol.Get("c"); p.Version.String() != "2.5.0" {
		t.Errorf("c = %s; the highest version both accept is 2.5.0", p.Version)
	}
}

// The scenario from the design discussion: A needs C ^20, B needs C ^25, and 20 is the newest C.
func TestConflictIsExplained(t *testing.T) {
	src := fake{
		"a": {"1.0.0": {"c": "^20.0"}},
		"b": {"1.0.0": {"c": "^25.0"}},
		"c": {"20.0.0": nil, "20.1.0": nil},
	}
	_, err := solve(t, src, "a", "*", "b", "*")
	var ce *ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("want ConflictError, got %v", err)
	}
	msg := err.Error()
	for _, want := range []string{"cannot find a version of c", "a@1.0.0 requires c ^20.0", "b@1.0.0 requires c ^25.0", "available versions: 20.1.0, 20.0.0"} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in:\n%s", want, msg)
		}
	}
}

func TestBacktracksToOlderVersion(t *testing.T) {
	// a 2.0.0 needs c ^2, but the root pins c ^1, so a must fall back to 1.0.0.
	src := fake{
		"a": {"2.0.0": {"c": "^2.0"}, "1.0.0": {"c": "^1.0"}},
		"c": {"1.5.0": nil, "2.0.0": nil},
	}
	sol, err := solve(t, src, "a", "*", "c", "^1.0")
	if err != nil || versions(sol) != "c@1.5.0 a@1.0.0" {
		t.Fatalf("%q %v", versions(sol), err)
	}
}

func TestBacktracksAcrossAlreadyChosenPackage(t *testing.T) {
	// c is chosen first (newest, 2.0.0); b then needs c ^1, so the search must
	// go back and choose c 1.x.
	src := fake{
		"b": {"1.0.0": {"c": "^1.0"}},
		"c": {"1.0.0": nil, "1.9.0": nil, "2.0.0": nil},
	}
	sol, err := solve(t, src, "c", "*", "b", "*")
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := sol.Get("c"); p.Version.String() != "1.9.0" {
		t.Errorf("c = %s, want 1.9.0", p.Version)
	}
}

func TestUnknownPackage(t *testing.T) {
	src := fake{"a": {"1.0.0": {"ghost": "*"}}}
	_, err := solve(t, src, "a", "*")
	var ce *ConflictError
	if !errors.As(err, &ce) || !ce.Unknown || !strings.Contains(err.Error(), "ghost does not exist (required by a@1.0.0)") {
		t.Errorf("%v", err)
	}
	// a nonexistent dependency of the newest version makes the solver use an older one
	src = fake{"a": {"2.0.0": {"ghost": "*"}, "1.0.0": nil}}
	if sol, err := solve(t, src, "a", "*"); err != nil || versions(sol) != "a@1.0.0" {
		t.Errorf("%q %v", versions(sol), err)
	}
}

func TestPrereleaseNeedsOptIn(t *testing.T) {
	src := fake{"a": {"1.0.0": nil, "2.0.0-rc.1": nil}}
	if sol, _ := solve(t, src, "a", "*"); versions(sol) != "a@1.0.0" {
		t.Errorf("%s", versions(sol))
	}
	if sol, err := solve(t, src, "a", ">=2.0.0-rc.1"); err != nil || versions(sol) != "a@2.0.0-rc.1" {
		t.Errorf("%s %v", versions(sol), err)
	}
}

func TestCycle(t *testing.T) {
	src := fake{"a": {"1.0.0": {"b": "*"}}, "b": {"1.0.0": {"a": "*"}}}
	_, err := solve(t, src, "a", "*")
	if err == nil || !strings.Contains(err.Error(), "dependency cycle: a -> b -> a") {
		t.Errorf("%v", err)
	}
}

func TestSourceErrorsAbort(t *testing.T) {
	boom := errors.New("network down")
	_, err := Solve(context.Background(), failing{boom}, roots(t, "a", "*"), Options{})
	if !errors.Is(err, boom) {
		t.Errorf("%v", err)
	}
}

type failing struct{ err error }

func (f failing) Versions(context.Context, string) ([]semver.Version, error) { return nil, f.err }
func (f failing) Dependencies(context.Context, string, semver.Version) ([]Dep, error) {
	return nil, f.err
}

func TestOrderHook(t *testing.T) {
	src := fake{"a": {"1.0.0": nil, "2.0.0": nil}}
	oldestFirst := func(_ string, c []semver.Version) []semver.Version {
		out := make([]semver.Version, len(c))
		for i, v := range c {
			out[len(c)-1-i] = v
		}
		return out
	}
	sol, err := Solve(context.Background(), src, roots(t, "a", "*"), Options{Order: oldestFirst})
	if err != nil || versions(sol) != "a@1.0.0" {
		t.Errorf("%s %v", versions(sol), err)
	}
}

func TestStepLimit(t *testing.T) {
	// Unsatisfiable chain with many versions: the budget must stop the search.
	src := fake{"root": {}, "x": {}}
	for i := 0; i < 30; i++ {
		src["root"][fmt.Sprintf("1.%d.0", i)] = map[string]string{"x": "^9.0"}
	}
	src["x"]["1.0.0"] = nil
	_, err := Solve(context.Background(), src, roots(t, "root", "*"), Options{MaxSteps: 5})
	if !errors.Is(err, ErrTooComplex) {
		t.Errorf("%v", err)
	}
}

func TestDeterministic(t *testing.T) {
	src := fake{
		"a": {"1.0.0": {"x": "*", "y": "*", "z": "*"}},
		"x": {"1.0.0": nil}, "y": {"1.0.0": nil}, "z": {"1.0.0": nil},
	}
	first, _ := solve(t, src, "a", "*")
	for i := 0; i < 50; i++ {
		again, _ := solve(t, src, "a", "*")
		if versions(again) != versions(first) {
			t.Fatalf("run %d: %s != %s", i, versions(again), versions(first))
		}
	}
}
