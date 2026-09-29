package networkscope

import "testing"

func TestResolveKeepsLegacyAdminAccessWhenUnset(t *testing.T) {
	scope, allowed, err := Resolve("", "admin", true)
	if err != nil || !allowed || !scope.All {
		t.Fatalf("Resolve() = %#v, %v, %v; want full legacy admin access", scope, allowed, err)
	}

	_, allowed, err = Resolve("", "viewer", false)
	if err != nil || allowed {
		t.Fatalf("non-admin without configuration must remain denied: allowed=%v err=%v", allowed, err)
	}
}

func TestResolveConfiguredScopesByLogin(t *testing.T) {
	scope, allowed, err := Resolve("admin=*;ops=192.168.18.1/24,10.32.22.0/24", "OPS", false)
	if err != nil || !allowed || scope.All {
		t.Fatalf("Resolve() = %#v, %v, %v; want restricted scope", scope, allowed, err)
	}
	if !scope.Allows("192.168.18.0/24") || !scope.Allows("10.32.22.0/24") {
		t.Fatalf("normalized configured networks are not allowed: %#v", scope.CIDRs)
	}
	if scope.Allows("172.16.0.0/16") {
		t.Fatal("unconfigured network unexpectedly allowed")
	}

	full, allowed, err := Resolve("admin=*;ops=192.168.18.0/24", "admin", true)
	if err != nil || !allowed || !full.All {
		t.Fatalf("wildcard scope = %#v, %v, %v; want full access", full, allowed, err)
	}

	_, allowed, err = Resolve("admin=*;ops=192.168.18.0/24", "missing", true)
	if err != nil || allowed {
		t.Fatalf("configured mode must deny unlisted users: allowed=%v err=%v", allowed, err)
	}
}

func TestResolveRejectsInvalidConfiguration(t *testing.T) {
	cases := []string{
		"ops",
		"=192.168.18.0/24",
		"ops=not-a-network",
		"ops=192.168.18.0/24;OPS=10.0.0.0/8",
		"ops=*,192.168.18.0/24",
	}
	for _, raw := range cases {
		if _, _, err := Resolve(raw, "ops", false); err == nil {
			t.Fatalf("Resolve(%q) succeeded; want validation error", raw)
		}
	}
}
