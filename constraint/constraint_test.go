package constraint

import (
	"testing"

	"github.com/pixelpolishers/kiln-core/semver"
)

func v(t *testing.T, s string) semver.Version {
	t.Helper()
	p, err := semver.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMatches(t *testing.T) {
	cases := []struct {
		constraint string
		yes, no    []string
	}{
		{"3.12.0", []string{"3.12.0"}, []string{"3.12.1", "3.11.9", "3.12.0-rc.1"}},
		{"=3.12.0", []string{"3.12.0"}, []string{"3.12.1"}},
		{"1.3", []string{"1.3.0"}, []string{"1.3.1", "1.4.0", "1.2.9"}}, // a plain version is exact (1.3 = 1.3.0); only 1.3.* is a range
		{"1", []string{"1.0.0"}, []string{"1.0.1", "1.1.0"}},
		{"*", []string{"0.0.1", "99.0.0"}, []string{"1.0.0-rc.1"}},
		{"3.*", []string{"3.0.0", "3.99.1"}, []string{"2.9.9", "4.0.0"}},
		{"3.12.*", []string{"3.12.0", "3.12.9"}, []string{"3.13.0", "3.11.9"}},
		{"^3.12", []string{"3.12.0", "3.99.0"}, []string{"3.11.9", "4.0.0"}},
		{"^1.2.3", []string{"1.2.3", "1.9.0"}, []string{"1.2.2", "2.0.0"}},
		{"^0.3", []string{"0.3.0", "0.3.9"}, []string{"0.4.0", "0.2.9"}},
		{"^0.0.3", []string{"0.0.3"}, []string{"0.0.4", "0.0.2"}},
		{"^0", []string{"0.5.0"}, []string{"1.0.0"}},
		{"~3.12", []string{"3.12.0", "3.99.0"}, []string{"3.11.0", "4.0.0"}},
		{"~3.12.1", []string{"3.12.1", "3.12.9"}, []string{"3.12.0", "3.13.0"}},
		{"~3", []string{"3.0.0", "3.9.0"}, []string{"4.0.0"}},
		{">1.0.0", []string{"1.0.1", "2.0.0"}, []string{"1.0.0", "0.9.0"}},
		{">=1.2", []string{"1.2.0", "2.0.0"}, []string{"1.1.9"}},
		{"<2.0", []string{"1.9.9"}, []string{"2.0.0", "2.1.0"}},
		{"<=2.0.0", []string{"2.0.0", "1.0.0"}, []string{"2.0.1"}},
		{"<=2", []string{"2.9.0"}, []string{"3.0.0"}},
		{">1", []string{"2.0.0"}, []string{"1.9.9"}},
		{"!=1.5.0", []string{"1.4.0", "1.5.1"}, []string{"1.5.0"}},
		{">=1.2 <2.0", []string{"1.2.0", "1.9.9"}, []string{"1.1.0", "2.0.0"}},
		{">=1.2,<2.0", []string{"1.5.0"}, []string{"2.0.0"}},
		{">= 1.2 , < 2.0", []string{"1.5.0"}, []string{"2.0.0"}},
		{">=1.0 <1.5 || >=2.0", []string{"1.2.0", "2.5.0"}, []string{"1.5.0", "1.9.0"}},
		{"^1.0 | ^3.0", []string{"1.5.0", "3.1.0"}, []string{"2.0.0"}},
		{"1.0 - 2.0", []string{"1.0.0", "2.0.5", "2.0.0"}, []string{"0.9.0", "2.1.0"}}, // partial upper bound includes the whole 2.0.x range
		{"1.* - 2.0", []string{"1.0.0", "2.0.9"}, []string{"0.9.0", "2.1.0"}},
		{"1.0.0 - 2.0.0", []string{"1.5.0", "2.0.0"}, []string{"2.0.1"}},
		{"^1.0.0-rc.1", []string{"1.0.0-rc.2", "1.0.0", "1.5.0"}, []string{"1.0.0-beta.1", "2.0.0"}},
		{"^1.0", nil, []string{"1.1.0-rc.1"}}, // pre-releases need an explicit opt-in
	}
	for _, c := range cases {
		con, err := Parse(c.constraint)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.constraint, err)
			continue
		}
		for _, s := range c.yes {
			if !con.Matches(v(t, s)) {
				t.Errorf("%q should match %s", c.constraint, s)
			}
		}
		for _, s := range c.no {
			if con.Matches(v(t, s)) {
				t.Errorf("%q should not match %s", c.constraint, s)
			}
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, s := range []string{"", "  ", "abc", "^", ">=", "1.2.3.4", "^1.x.2", "dev-main", "1.0-dev", "||", "^1.0 ||", "!=1.*", "1.0.* -rc"} {
		if c, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) = %q; want error", s, c)
		}
	}
}

func TestString(t *testing.T) {
	c, _ := Parse("  ^1.2 || ^3 ")
	if c.String() != "^1.2 || ^3" {
		t.Errorf("%q", c.String())
	}
	if !Any().Matches(v(t, "2.0.0")) {
		t.Error("Any must match")
	}
}
