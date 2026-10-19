package manifest

// AssetManifest is the typed form of kiln-assets.json (assets.schema.json):
// every binary published for one package version.
type AssetManifest struct {
	Package string      `json:"package"`
	Version string      `json:"version"`
	Source  AssetSource `json:"source"`
	Assets  []Asset     `json:"assets"`
	Schema  string      `json:"$schema,omitempty"`
}

type AssetSource struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	Archive    *File  `json:"archive,omitempty"`
}

// File is a downloadable file with its expected hash.
type File struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size,omitempty"`
}

type Attestation struct {
	Type     string `json:"type"`
	URL      string `json:"url,omitempty"`
	Identity string `json:"identity,omitempty"`
}

// AssetDep is the exact dependency build an asset was linked against.
type AssetDep struct {
	Version string `json:"version"`
	ID      string `json:"id"`
}

type Asset struct {
	ID           string              `json:"id"`
	Settings     map[string]any      `json:"settings"`
	Options      map[string]any      `json:"options,omitempty"`
	Dependencies map[string]AssetDep `json:"dependencies,omitempty"`
	File         File                `json:"file"`
	Attestation  Attestation         `json:"attestation"`
}
