// Package api defines the registry's JSON API types, shared by the registry
// server and the CLI client.
package api

import "encoding/json"

type PackageSummary struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	License      string `json:"license"`
	Latest       string `json:"latest"`
	VersionCount int    `json:"version_count"`
}

type VersionInfo struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	PublishedAt string `json:"published_at"`
	Flagged     bool   `json:"flagged"`
}

type PackageDetail struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	License     string        `json:"license"`
	Latest      string        `json:"latest"`
	Versions    []VersionInfo `json:"versions"`
}

type VersionDetail struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Commit      string          `json:"commit"`
	PublishedAt string          `json:"published_at"`
	Flagged     bool            `json:"flagged"`
	Manifest    json.RawMessage `json:"manifest"`
}

// Error is the body of every non-2xx API response.
type Error struct {
	Code    string `json:"error"`
	Message string `json:"message"`
}
