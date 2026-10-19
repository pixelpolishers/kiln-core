// Package solver picks one version of every package in a dependency graph so
// that all version constraints hold.
//
// It is a depth-first search with backtracking: packages are decided in the
// order they are discovered, each taking its newest version that satisfies
// every requirement collected so far; when a choice leads to a conflict the
// search returns to the most recent decision and tries the next version.
// Exactly one version per package is allowed, because two versions of a C++
// library cannot be linked into one program safely.
//
// When no solution exists the error describes the conflict closest to the
// root of the search (see ConflictError). It is not a full PubGrub
// implementation: explanations name the packages that disagree but do not
// derive a minimal proof.
package solver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/pixelpolishers/kiln-core/constraint"
	"github.com/pixelpolishers/kiln-core/semver"
)

// Dep is one dependency edge: a package and the versions acceptable to the dependent.
type Dep struct {
	Name       string
	Constraint constraint.Constraint
}

// Source supplies package metadata to the solver.
type Source interface {
	// Versions lists the installable versions of a package, in any order.
	// It returns an error wrapping ErrUnknownPackage if the package does not exist.
	Versions(ctx context.Context, name string) ([]semver.Version, error)
	// Dependencies lists the dependencies of one version. Callers filter out
	// dependencies that do not apply (other platforms, optional ones) before
	// returning them.
	Dependencies(ctx context.Context, name string, v semver.Version) ([]Dep, error)
}

var (
	ErrUnknownPackage = errors.New("unknown package")
	ErrTooComplex     = errors.New("dependency resolution gave up: step limit reached")

	errConflict = errors.New("conflict")
)

// Options tune the search. The zero value is usable.
type Options struct {
	// Order sorts the candidate versions of a package, most preferred first.
	// The default is newest first. This is where binary-aware solving plugs in.
	Order func(name string, candidates []semver.Version) []semver.Version
	// MaxSteps bounds the number of versions tried (default 200000).
	MaxSteps int
}

// Package is one selected package.
type Package struct {
	Name     string
	Version  semver.Version
	Requires []string // names of the packages it depends on, sorted
}

// Solution lists the selected packages with dependencies before dependents.
type Solution struct {
	Packages []Package
}

// Get returns the selected package with the given name.
func (s Solution) Get(name string) (Package, bool) {
	for _, p := range s.Packages {
		if p.Name == name {
			return p, true
		}
	}
	return Package{}, false
}

// Requirement is one constraint on a package, with the dependent that imposed it.
type Requirement struct {
	From       string // "root" or "name@version"
	Constraint string
}

// ConflictError means no version assignment satisfies all requirements.
type ConflictError struct {
	Package      string
	Unknown      bool // the package does not exist at all
	Requirements []Requirement
	Available    []semver.Version // newest first
	Selected     *semver.Version  // set if Package had already been chosen when the conflict appeared
}

func (e *ConflictError) Error() string {
	var b strings.Builder
	if e.Unknown {
		froms := make([]string, len(e.Requirements))
		for i, r := range e.Requirements {
			froms[i] = r.From
		}
		fmt.Fprintf(&b, "package %s does not exist (required by %s)", e.Package, strings.Join(froms, ", "))
		return b.String()
	}
	fmt.Fprintf(&b, "cannot find a version of %s that satisfies all requirements:", e.Package)
	for _, r := range e.Requirements {
		fmt.Fprintf(&b, "\n  %s requires %s %s", r.From, e.Package, r.Constraint)
	}
	if e.Selected != nil {
		fmt.Fprintf(&b, "\n  (%s %s had already been chosen)", e.Package, e.Selected)
	}
	if len(e.Available) == 0 {
		b.WriteString("\n  available versions: none")
	} else {
		vs := make([]string, len(e.Available))
		for i, v := range e.Available {
			vs[i] = v.String()
		}
		fmt.Fprintf(&b, "\n  available versions: %s", strings.Join(vs, ", "))
	}
	return b.String()
}

type req struct {
	from string
	c    constraint.Constraint
}

type search struct {
	ctx  context.Context
	src  Source
	opts Options

	selected map[string]semver.Version
	deps     map[string][]Dep // dependencies of the selected versions
	reqs     map[string][]req

	versionsCache map[string][]semver.Version
	unknown       map[string]bool
	depsCache     map[string][]Dep

	steps     int
	best      *ConflictError
	bestDepth int
}

// Solve finds a version for every package reachable from roots.
func Solve(ctx context.Context, src Source, roots []Dep, opts Options) (Solution, error) {
	if opts.MaxSteps == 0 {
		opts.MaxSteps = 200000
	}
	s := &search{
		ctx: ctx, src: src, opts: opts,
		selected:      map[string]semver.Version{},
		deps:          map[string][]Dep{},
		reqs:          map[string][]req{},
		versionsCache: map[string][]semver.Version{},
		unknown:       map[string]bool{},
		depsCache:     map[string][]Dep{},
	}
	var names []string
	for _, d := range roots {
		if _, seen := s.reqs[d.Name]; !seen {
			names = append(names, d.Name)
		}
		s.reqs[d.Name] = append(s.reqs[d.Name], req{"root", d.Constraint})
	}
	err := s.solve(names)
	if errors.Is(err, errConflict) {
		return Solution{}, s.best
	}
	if err != nil {
		return Solution{}, err
	}
	return s.order()
}

func (s *search) solve(pending []string) error {
	for len(pending) > 0 {
		if _, done := s.selected[pending[0]]; !done {
			break
		}
		pending = pending[1:]
	}
	if len(pending) == 0 {
		return nil
	}
	name, rest := pending[0], pending[1:]

	all, err := s.versions(name)
	if errors.Is(err, ErrUnknownPackage) {
		s.record(&ConflictError{Package: name, Unknown: true, Requirements: s.requirements(name)})
		return errConflict
	}
	if err != nil {
		return err
	}
	var cands []semver.Version
	for _, v := range all {
		if s.satisfiesAll(name, v) {
			cands = append(cands, v)
		}
	}
	if len(cands) == 0 {
		s.record(&ConflictError{Package: name, Requirements: s.requirements(name), Available: all})
		return errConflict
	}
	if s.opts.Order != nil {
		cands = s.opts.Order(name, cands)
	}

	for _, cand := range cands {
		if s.steps++; s.steps > s.opts.MaxSteps {
			return ErrTooComplex
		}
		deps, err := s.dependencies(name, cand)
		if err != nil {
			return err
		}
		from := name + "@" + cand.String()
		var pushed []string
		conflict := false
		for _, d := range deps {
			s.reqs[d.Name] = append(s.reqs[d.Name], req{from, d.Constraint})
			pushed = append(pushed, d.Name)
			if sel, ok := s.selected[d.Name]; ok && !d.Constraint.Matches(sel) {
				avail, _ := s.versions(d.Name)
				sel := sel
				s.record(&ConflictError{Package: d.Name, Requirements: s.requirements(d.Name), Available: avail, Selected: &sel})
				conflict = true
				break
			}
		}
		if conflict {
			s.pop(pushed)
			continue
		}

		s.selected[name] = cand
		s.deps[name] = deps
		next := append([]string(nil), rest...)
		for _, d := range deps {
			next = append(next, d.Name)
		}
		err = s.solve(next)
		if err == nil {
			return nil
		}
		delete(s.selected, name)
		delete(s.deps, name)
		s.pop(pushed)
		if !errors.Is(err, errConflict) {
			return err
		}
	}
	return errConflict
}

// record keeps the conflict found with the fewest packages chosen: it is the
// closest to the root requirements and so the most useful to show.
func (s *search) record(c *ConflictError) {
	if depth := len(s.selected); s.best == nil || depth < s.bestDepth {
		s.best, s.bestDepth = c, depth
	}
}

func (s *search) pop(pushed []string) {
	for i := len(pushed) - 1; i >= 0; i-- {
		n := pushed[i]
		s.reqs[n] = s.reqs[n][:len(s.reqs[n])-1]
	}
}

func (s *search) satisfiesAll(name string, v semver.Version) bool {
	for _, r := range s.reqs[name] {
		if !r.c.Matches(v) {
			return false
		}
	}
	return true
}

func (s *search) requirements(name string) []Requirement {
	out := make([]Requirement, len(s.reqs[name]))
	for i, r := range s.reqs[name] {
		out[i] = Requirement{From: r.from, Constraint: r.c.String()}
	}
	return out
}

// versions returns the versions of a package, newest first.
func (s *search) versions(name string) ([]semver.Version, error) {
	if s.unknown[name] {
		return nil, ErrUnknownPackage
	}
	if vs, ok := s.versionsCache[name]; ok {
		return vs, nil
	}
	vs, err := s.src.Versions(s.ctx, name)
	if errors.Is(err, ErrUnknownPackage) {
		s.unknown[name] = true
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	vs = append([]semver.Version(nil), vs...)
	sort.Slice(vs, func(i, j int) bool { return semver.Compare(vs[i], vs[j]) > 0 })
	s.versionsCache[name] = vs
	return vs, nil
}

func (s *search) dependencies(name string, v semver.Version) ([]Dep, error) {
	key := name + "@" + v.String()
	if d, ok := s.depsCache[key]; ok {
		return d, nil
	}
	d, err := s.src.Dependencies(s.ctx, name, v)
	if err != nil {
		return nil, fmt.Errorf("reading dependencies of %s: %w", key, err)
	}
	s.depsCache[key] = d
	return d, nil
}

// order sorts the solution so dependencies come before dependents and
// reports dependency cycles.
func (s *search) order() (Solution, error) {
	names := make([]string, 0, len(s.selected))
	for n := range s.selected {
		names = append(names, n)
	}
	sort.Strings(names)

	const (
		visiting = 1
		done     = 2
	)
	mark := map[string]int{}
	var sol Solution
	var path []string
	var visit func(n string) error
	visit = func(n string) error {
		switch mark[n] {
		case done:
			return nil
		case visiting:
			cycle := append(append([]string(nil), path...), n)
			for i, p := range cycle {
				if p == n {
					cycle = cycle[i:]
					break
				}
			}
			return fmt.Errorf("dependency cycle: %s", strings.Join(cycle, " -> "))
		}
		mark[n] = visiting
		path = append(path, n)
		requires := make([]string, 0, len(s.deps[n]))
		for _, d := range s.deps[n] {
			requires = append(requires, d.Name)
		}
		sort.Strings(requires)
		requires = dedupe(requires)
		for _, r := range requires {
			if err := visit(r); err != nil {
				return err
			}
		}
		path = path[:len(path)-1]
		mark[n] = done
		sol.Packages = append(sol.Packages, Package{Name: n, Version: s.selected[n], Requires: requires})
		return nil
	}
	for _, n := range names {
		if err := visit(n); err != nil {
			return Solution{}, err
		}
	}
	return sol, nil
}

func dedupe(sorted []string) []string {
	out := sorted[:0]
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			out = append(out, s)
		}
	}
	return out
}
