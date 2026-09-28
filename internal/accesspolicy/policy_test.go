package accesspolicy

import (
	"strings"
	"testing"
)

func TestAllocateIPv4(t *testing.T) {
	ip, err := AllocateIPv4("10.8.22.0/24", map[string]bool{"10.8.22.2": true})
	if err != nil {
		t.Fatal(err)
	}
	if ip != "10.8.22.3" {
		t.Fatalf("got %s, want 10.8.22.3", ip)
	}
}

func TestValidateStaticIPv4(t *testing.T) {
	for _, ip := range []string{"10.8.22.0", "10.8.22.1", "10.8.22.255", "10.8.23.2"} {
		if err := ValidateStaticIPv4("10.8.22.0/24", ip); err == nil {
			t.Fatalf("accepted reserved or out-of-pool address %s", ip)
		}
	}
	if err := ValidateStaticIPv4("10.8.22.0/24", "10.8.22.20"); err != nil {
		t.Fatal(err)
	}
}

func TestRenderCCD(t *testing.T) {
	got, err := RenderCCD("10.8.22.8", true, []string{"192.168.110.0/24", "192.168.10.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	want := "ifconfig-push 10.8.22.8 255.255.255.0\npush-remove \"route\"\npush \"route 192.168.10.0 255.255.255.0\"\npush \"route 192.168.110.0 255.255.255.0\"\n"
	if got != want {
		t.Fatalf("unexpected CCD:\n%s", got)
	}
	disabled, err := RenderCCD("10.8.22.8", false, nil)
	if err != nil || !strings.Contains(disabled, "disable\n") {
		t.Fatalf("disabled CCD: %q, %v", disabled, err)
	}
}

func TestCompileFirewallDefaultsToDeny(t *testing.T) {
	script, err := CompileFirewall("10.8.22.0/24", []UserPolicy{
		{IP: "10.8.22.8", Enabled: true, Networks: []string{"192.168.110.0/24"}},
		{IP: "10.8.22.9", Enabled: false, Networks: []string{"192.168.10.0/24"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"-A OPENVPN_UI_FORWARD -s 10.8.22.8/32 -d 192.168.110.0/24 -j ACCEPT",
		"-A OPENVPN_UI_INPUT -s 10.8.22.8/32 -d 192.168.110.0/24 -j ACCEPT",
		"-A OPENVPN_UI_FORWARD -s 10.8.22.0/24 -j DROP",
		"-A OPENVPN_UI_INPUT -s 10.8.22.0/24 -j DROP",
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("missing %q in:\n%s", fragment, script)
		}
	}
	if strings.Contains(script, "10.8.22.9/32") {
		t.Fatalf("disabled user was allowed:\n%s", script)
	}
}

func TestParseDiscoveredRoutes(t *testing.T) {
	raw := []byte(`[
      {"dst":"default","gateway":"10.0.0.1","dev":"eth0"},
      {"dst":"192.168.110.0/24","dev":"eth0","protocol":"kernel"},
      {"dst":"172.17.0.0/16","dev":"docker0"},
      {"dst":"10.8.22.0/24","dev":"tun0"},
      {"dst":"192.168.110.12/32","dev":"eth0"},
      {"dst":"203.0.113.0/24","dev":"eth0"}
    ]`)
	routes, err := ParseDiscoveredRoutes(raw, "10.8.22.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0].CIDR != "192.168.110.0/24" || routes[0].Interface != "eth0" {
		t.Fatalf("unexpected routes: %+v", routes)
	}
}
