package semver

import "testing"

func TestNormalize(t *testing.T) {
	ok := map[string]string{
		"1.3":          "1.3.0",
		"1.3.1":        "1.3.1",
		"11.0.2":       "11.0.2",
		"2.0.0-rc.1":   "2.0.0-rc.1",
		"1.2.3+build5": "1.2.3+build5",
	}
	for in, want := range ok {
		got, err := Normalize(in)
		if err != nil || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "1", "v1.2.3", "1.2.3.4", "01.2.3", "1.2.x", "1.1.1w"} {
		if got, err := Normalize(in); err == nil {
			t.Errorf("Normalize(%q) = %q; want error", in, got)
		}
	}
}

func TestCompare(t *testing.T) {
	order := []string{"0.9.0", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.3.0", "1.10.0", "2.0.0"}
	for i := range order {
		for j := range order {
			a, _ := Parse(order[i])
			b, _ := Parse(order[j])
			want := 0
			if i < j {
				want = -1
			} else if i > j {
				want = 1
			}
			if got := Compare(a, b); got != want {
				t.Errorf("Compare(%s, %s) = %d; want %d", order[i], order[j], got, want)
			}
		}
	}
	a, _ := Parse("1.0.0+a")
	b, _ := Parse("1.0.0+b")
	if Compare(a, b) != 0 {
		t.Error("build metadata must be ignored")
	}
}
