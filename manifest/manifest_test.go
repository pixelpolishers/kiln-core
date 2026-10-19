package manifest

import (
	"strings"
	"testing"
)

func TestValidatorAcceptsAManifestAndRefusesABrokenOne(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	good := `{"name":"acme/lib","license":"MIT","repository":{"type":"git","url":"https://example.com/acme/lib"},
	  "targets":{"lib":{"type":"header-only","include":["include"]}},
	  "build":{"system":"cmake"},"providers":[{"type":"github","tag":"v{version}"}]}`
	if _, _, err := v.Parse([]byte(good)); err != nil {
		t.Fatalf("a valid manifest was refused: %v", err)
	}
	bad := strings.Replace(good, `"header-only"`, `"banana"`, 1)
	if _, _, err := v.Parse([]byte(bad)); err == nil {
		t.Error("a manifest with an unknown target type must be refused")
	}
	if err := v.ValidateLock([]byte(`{"lock-version":2}`)); err == nil {
		t.Error("an incomplete lockfile must be refused")
	}
}
