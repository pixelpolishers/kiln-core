// Package semver parses and orders semantic versions. Two-part versions such
// as zlib's "1.3" are normalized to "1.3.0".
package semver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var re = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)(?:\.(0|[1-9]\d*))?(?:-([0-9A-Za-z.-]+))?(?:\+([0-9A-Za-z.-]+))?$`)

type Version struct {
	Major, Minor, Patch uint64
	Pre, Build          string
}

func Parse(s string) (Version, error) {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("invalid version %q", s)
	}
	var v Version
	var err error
	if v.Major, err = strconv.ParseUint(m[1], 10, 64); err != nil {
		return Version{}, fmt.Errorf("invalid version %q: %w", s, err)
	}
	if v.Minor, err = strconv.ParseUint(m[2], 10, 64); err != nil {
		return Version{}, fmt.Errorf("invalid version %q: %w", s, err)
	}
	if m[3] != "" {
		if v.Patch, err = strconv.ParseUint(m[3], 10, 64); err != nil {
			return Version{}, fmt.Errorf("invalid version %q: %w", s, err)
		}
	}
	v.Pre, v.Build = m[4], m[5]
	return v, nil
}

func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

func (v Version) IsPrerelease() bool { return v.Pre != "" }

// Normalize returns the canonical three-part form of s.
func Normalize(s string) (string, error) {
	v, err := Parse(s)
	if err != nil {
		return "", err
	}
	return v.String(), nil
}

// Compare orders versions per semver precedence (build metadata is ignored).
func Compare(a, b Version) int {
	for _, p := range [][2]uint64{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if p[0] != p[1] {
			if p[0] < p[1] {
				return -1
			}
			return 1
		}
	}
	switch {
	case a.Pre == b.Pre:
		return 0
	case a.Pre == "":
		return 1
	case b.Pre == "":
		return -1
	}
	return comparePre(a.Pre, b.Pre)
}

func comparePre(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		an, aerr := strconv.ParseUint(as[i], 10, 64)
		bn, berr := strconv.ParseUint(bs[i], 10, 64)
		switch {
		case aerr == nil && berr == nil:
			if an != bn {
				if an < bn {
					return -1
				}
				return 1
			}
		case aerr == nil:
			return -1
		case berr == nil:
			return 1
		default:
			if c := strings.Compare(as[i], bs[i]); c != 0 {
				return c
			}
		}
	}
	switch {
	case len(as) < len(bs):
		return -1
	case len(as) > len(bs):
		return 1
	}
	return 0
}
