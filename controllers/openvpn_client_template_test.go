package controllers

import (
	"os"
	"strings"
	"testing"

	clientconfig "github.com/d3vilh/openvpn-server-config/client/client-config"
)

func TestClientProfileDoesNotRequirePlatformSpecificUserOrGroup(t *testing.T) {
	templateData, err := os.ReadFile("../conf/openvpn-client-config.tpl")
	if err != nil {
		t.Fatal(err)
	}
	profile, err := GetText(string(templateData), clientconfig.Config{
		OVClientUser:  "nobody",
		OVClientGroup: "nogroup",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, directive := range []string{"user nobody", "group nogroup"} {
		if strings.Contains(profile, directive) {
			t.Fatalf("client profile contains platform-specific directive %q", directive)
		}
	}
}
