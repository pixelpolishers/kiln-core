package assetid

import "testing"

func base() Input {
	return Input{
		Package: "gabime/spdlog", Version: "1.14.1",
		Settings: map[string]any{"os": "linux", "arch": "x86_64", "build_type": "release", "compiler": map[string]any{"name": "gcc", "version": "13"}},
		Options:  map[string]any{"header_only": false},
		Deps:     map[string]Dep{"fmtlib/fmt": {"11.0.2", "aaaa"}},
	}
}

func id(t *testing.T, in Input) string {
	t.Helper()
	s, err := Compute(in)
	if err != nil || len(s) != 64 {
		t.Fatalf("%q %v", s, err)
	}
	return s
}

func TestStableAndSensitive(t *testing.T) {
	want := id(t, base())
	if id(t, base()) != want {
		t.Fatal("not deterministic")
	}
	changes := map[string]func(*Input){
		"package":      func(i *Input) { i.Package = "other/pkg" },
		"version":      func(i *Input) { i.Version = "1.14.2" },
		"setting":      func(i *Input) { i.Settings["build_type"] = "debug" },
		"nested":       func(i *Input) { i.Settings["compiler"] = map[string]any{"name": "gcc", "version": "14"} },
		"option":       func(i *Input) { i.Options["header_only"] = true },
		"dep version":  func(i *Input) { i.Deps["fmtlib/fmt"] = Dep{"11.0.3", "aaaa"} },
		"dep build":    func(i *Input) { i.Deps["fmtlib/fmt"] = Dep{"11.0.2", "bbbb"} },
		"extra dep":    func(i *Input) { i.Deps["zlib/zlib"] = Dep{"1.3.0", "cccc"} },
		"removed dep":  func(i *Input) { i.Deps = nil },
		"extra option": func(i *Input) { i.Options["x"] = 1 },
	}
	for name, mutate := range changes {
		in := base()
		mutate(&in)
		if id(t, in) == want {
			t.Errorf("changing %s must change the id", name)
		}
	}
}

func TestKeyOrderAndNilEquivalence(t *testing.T) {
	a := Input{Package: "a/b", Version: "1.0.0", Settings: map[string]any{"os": "linux", "arch": "x86_64"}}
	b := Input{Package: "a/b", Version: "1.0.0", Settings: map[string]any{"arch": "x86_64", "os": "linux"}, Options: map[string]any{}, Deps: map[string]Dep{}}
	if id(t, a) != id(t, b) {
		t.Error("map order and nil vs empty must not matter")
	}
}

func TestRelevant(t *testing.T) {
	full := map[string]any{"os": "linux", "arch": "x86_64", "stdlib": "libstdc++", "cpp_standard": "17", "cxx11_abi": true, "build_type": "release", "linkage": "static"}

	if got := Relevant(full, Kind{Language: "c++", HeaderOnly: true}); len(got) != 0 {
		t.Errorf("header-only must have no settings: %v", got)
	}
	c := Relevant(full, Kind{Language: "c"})
	for _, k := range []string{"stdlib", "cpp_standard", "cxx11_abi"} {
		if _, ok := c[k]; ok {
			t.Errorf("C package must ignore %s", k)
		}
	}
	if c["os"] != "linux" || c["build_type"] != "release" || c["linkage"] != "static" {
		t.Errorf("C package must keep the rest: %v", c)
	}
	if got := Relevant(full, Kind{Language: "c++"}); len(got) != len(full) {
		t.Errorf("C++ package keeps everything: %v", got)
	}
	if _, ok := full["stdlib"]; !ok {
		t.Error("input must not be modified")
	}

	// the practical consequence: a C library's id is the same under any C++ standard
	other := map[string]any{}
	for k, v := range full {
		other[k] = v
	}
	other["cpp_standard"] = "20"
	i1 := Input{Package: "madler/zlib", Version: "1.3.1", Settings: Relevant(full, Kind{Language: "c"})}
	i2 := Input{Package: "madler/zlib", Version: "1.3.1", Settings: Relevant(other, Kind{Language: "c"})}
	if id(t, i1) != id(t, i2) {
		t.Error("zlib must not split by C++ standard")
	}
}
