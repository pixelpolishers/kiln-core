// Package constraint parses version constraints (exact, wildcard, caret, tilde, ranges).
//
// Supported syntax:
//
//	3.12.0  =3.12.0      exact (a two-part "1.3" means 1.3.0)
//	*  3.*  3.12.*       wildcards
//	^3.12  ^0.3  ^0.0.3  caret: compatible with, per semver
//	~3.12  ~3.12.1       tilde: ~1.2 is >=1.2.0 <2.0.0, ~1.2.3 is >=1.2.3 <1.3.0
//	>1 >=1.2 <2 <=2 !=1.5  comparison operators
//	>=1.2 <2.0  >=1.2,<2.0   AND (space or comma)
//	^1.0 || ^2.0  ^1.0 | ^2.0   OR
//	1.0 - 2.0            hyphen range: inclusive; a partial upper bound includes its whole range
//
// Pre-release versions satisfy a constraint only if the constraint itself
// names a pre-release. "dev-" branch constraints are not supported yet.
package constraint

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pixelpolishers/kiln-core/semver"
)

type op int

const (
	opEQ op = iota
	opNE
	opGT
	opGE
	opLT
	opLE
)

type comparator struct {
	op op
	v  semver.Version
}

func (c comparator) matches(v semver.Version) bool {
	cmp := semver.Compare(v, c.v)
	switch c.op {
	case opEQ:
		return cmp == 0
	case opNE:
		return cmp != 0
	case opGT:
		return cmp > 0
	case opGE:
		return cmp >= 0
	case opLT:
		return cmp < 0
	default:
		return cmp <= 0
	}
}

// Constraint is a disjunction (OR) of conjunctions (AND) of comparators.
type Constraint struct {
	raw    string
	groups [][]comparator
	// preOK is true when the constraint names a pre-release, which allows
	// pre-release versions to match.
	preOK bool
}

func (c Constraint) String() string { return c.raw }

// Any returns the constraint that matches every stable version.
func Any() Constraint {
	c, _ := Parse("*")
	return c
}

// Matches reports whether v satisfies the constraint.
func (c Constraint) Matches(v semver.Version) bool {
	if v.IsPrerelease() && !c.preOK {
		return false
	}
	for _, g := range c.groups {
		ok := true
		for _, cmp := range g {
			if !cmp.matches(v) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

var (
	reOr     = regexp.MustCompile(`\s*\|\|?\s*`)
	reHyphen = regexp.MustCompile(`^\s*(\S+)\s+-\s+(\S+)\s*$`)
	reAnd    = regexp.MustCompile(`[\s,]+`)
	reOp     = regexp.MustCompile(`^(>=|<=|!=|==|=|>|<|\^|~)?\s*(.+)$`)
)

// Parse parses a constraint string.
func Parse(s string) (Constraint, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Constraint{}, fmt.Errorf("empty version constraint")
	}
	if strings.HasPrefix(raw, "dev-") || strings.HasSuffix(raw, "-dev") {
		return Constraint{}, fmt.Errorf("invalid constraint %q: dev versions are not supported yet", s)
	}
	c := Constraint{raw: raw}
	for _, alt := range reOr.Split(raw, -1) {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			return Constraint{}, fmt.Errorf("invalid constraint %q: empty alternative around ||", s)
		}
		group, err := parseGroup(alt)
		if err != nil {
			return Constraint{}, fmt.Errorf("invalid constraint %q: %w", s, err)
		}
		for _, cmp := range group {
			if cmp.v.IsPrerelease() {
				c.preOK = true
			}
		}
		c.groups = append(c.groups, group)
	}
	return c, nil
}

func parseGroup(s string) ([]comparator, error) {
	if m := reHyphen.FindStringSubmatch(s); m != nil {
		lo, err := parsePartial(m[1])
		if err != nil {
			return nil, err
		}
		hi, err := parsePartial(m[2])
		if err != nil {
			return nil, err
		}
		upper := comparator{opLE, hi.version()}
		if hi.parts < 3 {
			upper = comparator{opLT, hi.next()}
		}
		return []comparator{{opGE, lo.version()}, upper}, nil
	}

	// Join "op version" written with a space ("> 1.0") before splitting on spaces.
	s = regexp.MustCompile(`(>=|<=|!=|==|=|>|<|\^|~)\s+`).ReplaceAllString(s, "$1")
	var out []comparator
	for _, tok := range reAnd.Split(s, -1) {
		if tok == "" {
			continue
		}
		cmps, err := parseToken(tok)
		if err != nil {
			return nil, err
		}
		out = append(out, cmps...)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no version found")
	}
	return out, nil
}

func parseToken(tok string) ([]comparator, error) {
	m := reOp.FindStringSubmatch(tok)
	if m == nil {
		return nil, fmt.Errorf("cannot parse %q", tok)
	}
	opS, rest := m[1], m[2]
	p, err := parsePartial(rest)
	if err != nil {
		return nil, err
	}
	switch opS {
	case "^":
		return []comparator{{opGE, p.version()}, {opLT, p.caretUpper()}}, nil
	case "~":
		return []comparator{{opGE, p.version()}, {opLT, p.tildeUpper()}}, nil
	case ">=":
		return []comparator{{opGE, p.version()}}, nil
	case "<=":
		if p.parts < 3 {
			return []comparator{{opLT, p.next()}}, nil
		}
		return []comparator{{opLE, p.version()}}, nil
	case ">":
		if p.parts < 3 {
			return []comparator{{opGE, p.next()}}, nil
		}
		return []comparator{{opGT, p.version()}}, nil
	case "<":
		return []comparator{{opLT, p.version()}}, nil
	case "!=":
		if p.wild || p.parts == 0 {
			return nil, fmt.Errorf("!= cannot be combined with a wildcard")
		}
		return []comparator{{opNE, p.version()}}, nil
	default: // "", "=", "=="
		if p.wild {
			return p.wildRange(), nil
		}
		return []comparator{{opEQ, p.version()}}, nil
	}
}

// partial is a version with possibly missing or wildcard components.
type partial struct {
	major, minor, patch uint64
	pre                 string
	parts               int  // number of numeric components given (0 for "*")
	wild                bool // a trailing wildcard was given
}

var rePartial = regexp.MustCompile(`^v?(\*|[xX]|\d+)(?:\.(\*|[xX]|\d+))?(?:\.(\*|[xX]|\d+))?(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

func parsePartial(s string) (partial, error) {
	m := rePartial.FindStringSubmatch(s)
	if m == nil {
		return partial{}, fmt.Errorf("%q is not a version", s)
	}
	var p partial
	p.pre = m[4]
	nums := []*uint64{&p.major, &p.minor, &p.patch}
	for i := 0; i < 3; i++ {
		part := m[i+1]
		if part == "" {
			break
		}
		if part == "*" || part == "x" || part == "X" {
			p.wild = true
			for _, rest := range m[i+2 : 4] {
				if rest != "" && rest != "*" && rest != "x" && rest != "X" {
					return partial{}, fmt.Errorf("%q: only wildcards may follow a wildcard", s)
				}
			}
			break
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return partial{}, fmt.Errorf("%q is not a version", s)
		}
		*nums[i] = n
		p.parts = i + 1
	}
	if p.wild && p.pre != "" {
		return partial{}, fmt.Errorf("%q: a wildcard cannot have a pre-release", s)
	}
	return p, nil
}

func (p partial) version() semver.Version {
	return semver.Version{Major: p.major, Minor: p.minor, Patch: p.patch, Pre: p.pre}
}

// next is the first version past the range the partial describes
// (1 -> 2.0.0, 1.2 -> 1.3.0, 1.2.3 -> 1.2.4).
func (p partial) next() semver.Version {
	switch p.parts {
	case 0:
		return semver.Version{Major: ^uint64(0)}
	case 1:
		return semver.Version{Major: p.major + 1}
	case 2:
		return semver.Version{Major: p.major, Minor: p.minor + 1}
	}
	return semver.Version{Major: p.major, Minor: p.minor, Patch: p.patch + 1}
}

// wildRange expands "*", "3.*" or "3.12.*" (and bare "3", "3.12") to a range.
func (p partial) wildRange() []comparator {
	if p.parts == 0 {
		return []comparator{{opGE, semver.Version{}}}
	}
	return []comparator{{opGE, p.version()}, {opLT, p.next()}}
}

func (p partial) caretUpper() semver.Version {
	switch {
	case p.parts == 0:
		return semver.Version{Major: ^uint64(0)}
	case p.major > 0 || p.parts == 1:
		return semver.Version{Major: p.major + 1}
	case p.minor > 0 || p.parts == 2:
		return semver.Version{Major: 0, Minor: p.minor + 1}
	}
	return semver.Version{Major: 0, Minor: 0, Patch: p.patch + 1}
}

func (p partial) tildeUpper() semver.Version {
	if p.parts <= 2 {
		return semver.Version{Major: p.major + 1}
	}
	return semver.Version{Major: p.major, Minor: p.minor + 1}
}
