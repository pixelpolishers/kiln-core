package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Package is the typed view of a kiln.json that the CLI works with. It is
// decoded after the document has passed schema validation, so it only models
// the fields the tools use.
type Package struct {
	Name        string                `json:"name"`
	Version     string                `json:"version"`
	Description string                `json:"description"`
	License     string                `json:"license"`
	Language    string                `json:"language"`
	Repository  *Repository           `json:"repository"`
	Require     map[string]Dependency `json:"require"`
	RequireDev  map[string]Dependency `json:"require-dev"`
	Options     map[string]Option     `json:"options"`
	Targets     map[string]Target     `json:"targets"`
	Build       *Build                `json:"build"`
	// SystemRequires is what the machine must have to use the package (and to build it).
	SystemRequires []Requirement `json:"system-requires"`
	Providers      []Provider    `json:"providers"`
}

type Repository struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	// Submodules asks for the repository's git submodules to be checked out with the source.
	Submodules bool `json:"submodules"`
}

// Dependency is a version constraint, optionally with extra fields. In JSON it
// is either a string or an object.
type Dependency struct {
	Version   string         `json:"version"`
	Options   map[string]any `json:"options"`
	Optional  bool           `json:"optional"`
	Platforms []string       `json:"platforms"`
	Targets   []string       `json:"targets"`
}

func (d *Dependency) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*d = Dependency{Version: s}
		return nil
	}
	type plain Dependency
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return fmt.Errorf("dependency must be a string or an object: %w", err)
	}
	*d = Dependency(p)
	return nil
}

type Option struct {
	Description string `json:"description"`
	Default     any    `json:"default"`
	Values      []any  `json:"values"`
}

type Target struct {
	Type          string         `json:"type"`
	When          map[string]any `json:"when"`
	OutputName    string         `json:"output-name"`
	Include       []Conditional  `json:"include"`
	Defines       map[string]any `json:"defines"`
	LinkLibraries []Conditional  `json:"link-libraries"`
	Requires      []string       `json:"requires"`
}

// Conditional is a string that applies only when its condition holds. In JSON
// it is either a plain string or {"value": ..., "when": {...}}.
type Conditional struct {
	Value string         `json:"value"`
	When  map[string]any `json:"when"`
}

func (c *Conditional) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*c = Conditional{Value: s}
		return nil
	}
	type plain Conditional
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return fmt.Errorf("must be a string or {value, when}: %w", err)
	}
	*c = Conditional(p)
	return nil
}

type Build struct {
	System    string                 `json:"system"`
	SourceDir string                 `json:"source-dir"`
	Options   map[string]OptionValue `json:"options"`
	Commands  []Command              `json:"commands"`
	Requires  []Requirement          `json:"requires"`
}

// Requirement is something the machine must have to build or use a package.
// Exactly one of Tool, Check, PkgConfig, File and Env is set.
type Requirement struct {
	Name      string   `json:"name"`
	Tool      string   `json:"tool"`
	Check     []string `json:"check"`
	PkgConfig string   `json:"pkg-config"`
	// File is a path (a glob is allowed, $NAME and ${NAME} are expanded) that must exist.
	File string `json:"file"`
	// Env is an environment variable that must be set and not empty.
	Env      string         `json:"env"`
	Optional bool           `json:"optional"`
	Hint     Hint           `json:"hint"`
	When     map[string]any `json:"when"`
}

// Hint is install advice: a plain string, or advice per distribution or
// operating system.
type Hint struct {
	Default string
	By      map[string]string
}

func (h *Hint) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*h = Hint{Default: s}
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("hint must be a string or an object of strings: %w", err)
	}
	*h = Hint{By: m, Default: m["default"]}
	return nil
}

// Command is one step of a "custom" build.
type Command struct {
	Run  []Conditional  `json:"run"`
	When map[string]any `json:"when"`
}

// OptionValue is a build option: a scalar, or an ordered list of cases of which
// the first whose condition holds applies.
type OptionValue struct {
	Cases []OptionCase
}

type OptionCase struct {
	Value any            `json:"value"`
	When  map[string]any `json:"when"`
}

func (o *OptionValue) UnmarshalJSON(b []byte) error {
	var cases []OptionCase
	if trimmed := bytes.TrimSpace(b); len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(b, &cases); err != nil {
			return fmt.Errorf("option value list: %w", err)
		}
		o.Cases = cases
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Cases = []OptionCase{{Value: v}}
	return nil
}

type Provider struct {
	Type       string `json:"type"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	URL        string `json:"url"`
	Path       string `json:"path"`
}

// ParsePackage decodes an already validated manifest.
func ParsePackage(raw []byte) (Package, error) {
	var p Package
	if err := json.Unmarshal(raw, &p); err != nil {
		return Package{}, err
	}
	return p, nil
}
