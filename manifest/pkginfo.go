package manifest

// PackageInfoFile is the name of the metadata file inside every package
// archive. It describes the built package in resolved form (no conditions, no
// placeholders), so a consumer can generate build-system integration without
// asking the registry.
const PackageInfoFile = "kiln-package.json"

type PackageInfo struct {
	Name    string                `json:"name"`
	Version string                `json:"version"`
	Targets map[string]TargetInfo `json:"targets"`
}

type TargetInfo struct {
	Type string `json:"type"`
	// Include are directories relative to the package root.
	Include []string `json:"include,omitempty"`
	// Libs are library files relative to the package root.
	Libs []string `json:"libs,omitempty"`
	// Exe is the program file of an executable target, relative to the package root.
	Exe     string         `json:"exe,omitempty"`
	Defines map[string]any `json:"defines,omitempty"`
	// Links are system libraries to link (e.g. "pthread").
	Links []string `json:"links,omitempty"`
	// Uses are sibling targets of the same package this target links.
	Uses []string `json:"uses,omitempty"`
	// Requires are the packages (and their targets) this target links.
	Requires []TargetRequire `json:"requires,omitempty"`
}

// TargetRequire names a dependency package; Targets empty means all of its targets.
type TargetRequire struct {
	Package string   `json:"package"`
	Targets []string `json:"targets,omitempty"`
}
