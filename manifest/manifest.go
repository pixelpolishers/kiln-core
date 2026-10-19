// Package manifest validates kiln.json documents against the embedded JSON
// schemas and produces their canonical form.
package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/santhosh-tekuri/jsonschema/v6"

	schemas "github.com/pixelpolishers/kiln-schema"
)

const schemaBase = "https://kiln.local/schemas/"

type Validator struct {
	pkg      *jsonschema.Schema
	settings *jsonschema.Schema
	assets   *jsonschema.Schema
	lock     *jsonschema.Schema
}

func NewValidator() (*Validator, error) {
	c := jsonschema.NewCompiler()
	entries, err := fs.ReadDir(schemas.FS, ".")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		f, err := schemas.FS.Open(e.Name())
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if err := c.AddResource(schemaBase+e.Name(), doc); err != nil {
			return nil, err
		}
	}
	pkg, err := c.Compile(schemaBase + "package.schema.json")
	if err != nil {
		return nil, err
	}
	settings, err := c.Compile(schemaBase + "settings.schema.json")
	if err != nil {
		return nil, err
	}
	assets, err := c.Compile(schemaBase + "assets.schema.json")
	if err != nil {
		return nil, err
	}
	lock, err := c.Compile(schemaBase + "lock.schema.json")
	if err != nil {
		return nil, err
	}
	return &Validator{pkg: pkg, settings: settings, assets: assets, lock: lock}, nil
}

func validateRaw(s *jsonschema.Schema, what string, raw []byte) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("%s is not valid JSON: %w", what, err)
	}
	return s.Validate(doc)
}

// ValidateLock checks a lockfile document (see lock.schema.json).
func (v *Validator) ValidateLock(raw []byte) error { return validateRaw(v.lock, "lockfile", raw) }

// ParseAssets validates an asset manifest and decodes it.
func (v *Validator) ParseAssets(raw []byte) (AssetManifest, error) {
	if err := validateRaw(v.assets, "asset manifest", raw); err != nil {
		return AssetManifest{}, err
	}
	var m AssetManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return AssetManifest{}, err
	}
	return m, nil
}

// ValidateSettings checks a build settings document (see settings.schema.json).
func (v *Validator) ValidateSettings(raw []byte) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("settings are not valid JSON: %w", err)
	}
	return v.settings.Validate(doc)
}

// Manifest holds the fields the registry needs; the full document is kept as
// canonical JSON.
type Manifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	License     string `json:"license"`
}

// Parse validates raw against the package schema. It returns the decoded
// summary fields and the canonical JSON encoding (sorted keys, compact).
func (v *Validator) Parse(raw []byte) (Manifest, []byte, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("manifest is not valid JSON: %w", err)
	}
	if err := v.pkg.Validate(doc); err != nil {
		return Manifest{}, nil, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, nil, err
	}
	canon, err := Canonical(doc)
	return m, canon, err
}

// Canonical encodes a decoded JSON value deterministically: object keys are
// sorted, there is no insignificant whitespace and HTML is not escaped.
func Canonical(doc any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
