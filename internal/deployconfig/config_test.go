package deployconfig

import "testing"

func TestDeploymentDefaults(t *testing.T) {
	c, err := Read(func(k string) string {
		return map[string]string{
			"OPENVPN_PUBLIC_HOST": "vpn.example.com", "OPENVPN_PUBLIC_PORT": "21194",
			"OPENVPN_VPN_CIDR": "10.8.0.0/24", "OPENVPN_LAN_CIDR": "192.168.18.0/24",
		}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "vpn.example.com" || c.Port != "21194" || c.Protocol != "tcp" || c.Server != "server 10.8.0.0 255.255.255.0" || c.PushRoute != `push "route 192.168.18.0 255.255.255.0"` {
		t.Fatalf("unexpected config: %+v", c)
	}
}

func TestInvalidDeploymentSettings(t *testing.T) {
	for _, pair := range [][2]string{
		{"OPENVPN_PUBLIC_HOST", "https://vpn.example.com"},
		{"OPENVPN_PUBLIC_HOST", "vpn.example.com\npush bad"},
		{"OPENVPN_PUBLIC_PORT", "65536"}, {"OPENVPN_PUBLIC_PORT", "0"},
		{"OPENVPN_PROTOCOL", "tcp-server"},
		{"OPENVPN_VPN_CIDR", "10.8.0.1/24"}, {"OPENVPN_VPN_CIDR", "10.8.0.0/16"},
		{"OPENVPN_LAN_CIDR", "::/64"}, {"OPENVPN_LAN_CIDR", "10.8.0.0/16"},
	} {
		t.Run(pair[0]+pair[1], func(t *testing.T) {
			_, err := Read(func(k string) string {
				if k == pair[0] {
					return pair[1]
				}
				if k == "OPENVPN_PUBLIC_HOST" {
					return "vpn.example.com"
				}
				return ""
			})
			if err == nil {
				t.Fatal("invalid setting accepted")
			}
		})
	}
}

func TestNoDeploymentEnvironment(t *testing.T) {
	c, err := Read(func(string) string { return "" })
	if err != nil || c.Host != "" {
		t.Fatalf("normal source startup changed: %+v, %v", c, err)
	}
}
