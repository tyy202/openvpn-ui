package appversion

import "testing"

func TestCurrentNormalizesEmbeddedVersion(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = " 0.5.0\n"
	if got := Current(); got != "0.5.0" {
		t.Fatalf("Current() = %q, want 0.5.0", got)
	}

	Version = " "
	if got := Current(); got != "dev" {
		t.Fatalf("Current() = %q, want dev fallback", got)
	}
}
