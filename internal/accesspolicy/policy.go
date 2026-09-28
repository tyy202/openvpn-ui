package accesspolicy

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
)

type UserPolicy struct {
	IP       string
	Enabled  bool
	Networks []string
}

type DiscoveredRoute struct {
	CIDR      string
	Interface string
	Gateway   string
}

func pool(cidr string) (*net.IPNet, net.IP, error) {
	ip, n, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil || ip.To4() == nil {
		return nil, nil, fmt.Errorf("invalid IPv4 CIDR %q", cidr)
	}
	n.IP = ip.To4()
	return n, n.IP, nil
}

func ipv4Uint(ip net.IP) uint32 {
	v := ip.To4()
	return uint32(v[0])<<24 | uint32(v[1])<<16 | uint32(v[2])<<8 | uint32(v[3])
}

func uintIPv4(v uint32) net.IP {
	return net.IPv4(byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func ValidateStaticIPv4(cidr, address string) error {
	n, first, err := pool(cidr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(strings.TrimSpace(address)).To4()
	if ip == nil || !n.Contains(ip) {
		return fmt.Errorf("address %q is outside VPN pool %s", address, cidr)
	}
	ones, bits := n.Mask.Size()
	if bits != 32 || ones > 30 {
		return fmt.Errorf("VPN pool %s has no usable client addresses", cidr)
	}
	base := ipv4Uint(first)
	last := base | ^ipv4Uint(net.IP(n.Mask))
	v := ipv4Uint(ip)
	if v == base || v == base+1 || v == last {
		return fmt.Errorf("address %s is reserved", address)
	}
	return nil
}

func AllocateIPv4(cidr string, used map[string]bool) (string, error) {
	n, first, err := pool(cidr)
	if err != nil {
		return "", err
	}
	ones, bits := n.Mask.Size()
	if bits != 32 || ones > 30 {
		return "", fmt.Errorf("VPN pool %s has no usable client addresses", cidr)
	}
	base := ipv4Uint(first)
	last := base | ^ipv4Uint(net.IP(n.Mask))
	for value := base + 2; value < last; value++ {
		candidate := uintIPv4(value).String()
		if !used[candidate] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("VPN pool %s is exhausted", cidr)
}

func normalizeCIDR(cidr string) (string, *net.IPNet, error) {
	n, _, err := pool(cidr)
	if err != nil {
		return "", nil, err
	}
	return n.String(), n, nil
}

func RenderCCD(address string, enabled bool, networks []string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(address)).To4()
	if ip == nil {
		return "", fmt.Errorf("invalid fixed IPv4 address %q", address)
	}
	unique := map[string]bool{}
	for _, item := range networks {
		cidr, _, err := normalizeCIDR(item)
		if err != nil {
			return "", err
		}
		unique[cidr] = true
	}
	ordered := make([]string, 0, len(unique))
	for cidr := range unique {
		ordered = append(ordered, cidr)
	}
	sort.Strings(ordered)

	var b strings.Builder
	b.WriteString("ifconfig-push " + ip.String() + " 255.255.255.0\n")
	if !enabled {
		b.WriteString("disable\n")
		return b.String(), nil
	}
	b.WriteString("push-remove \"route\"\n")
	for _, cidr := range ordered {
		_, n, _ := net.ParseCIDR(cidr)
		b.WriteString(fmt.Sprintf("push \"route %s %s\"\n", n.IP.String(), net.IP(n.Mask).String()))
	}
	return b.String(), nil
}

func CompileFirewall(vpnCIDR string, policies []UserPolicy) (string, error) {
	cidr, _, err := normalizeCIDR(vpnCIDR)
	if err != nil {
		return "", err
	}
	type rule struct{ source, destination string }
	rules := make([]rule, 0)
	seen := map[string]bool{}
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		if err := ValidateStaticIPv4(cidr, policy.IP); err != nil {
			return "", err
		}
		for _, network := range policy.Networks {
			destination, _, err := normalizeCIDR(network)
			if err != nil {
				return "", err
			}
			key := policy.IP + "\x00" + destination
			if !seen[key] {
				seen[key] = true
				rules = append(rules, rule{policy.IP, destination})
			}
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].source == rules[j].source {
			return rules[i].destination < rules[j].destination
		}
		return rules[i].source < rules[j].source
	})

	var b strings.Builder
	b.WriteString("#!/bin/sh\nset -eu\n")
	b.WriteString("iptables -N OPENVPN_UI_FORWARD 2>/dev/null || true\n")
	b.WriteString("iptables -N OPENVPN_UI_INPUT 2>/dev/null || true\n")
	b.WriteString("iptables -C FORWARD -s " + cidr + " -j OPENVPN_UI_FORWARD 2>/dev/null || iptables -I FORWARD 1 -s " + cidr + " -j OPENVPN_UI_FORWARD\n")
	b.WriteString("iptables -C INPUT -s " + cidr + " -j OPENVPN_UI_INPUT 2>/dev/null || iptables -I INPUT 1 -s " + cidr + " -j OPENVPN_UI_INPUT\n")
	b.WriteString("iptables-restore --noflush <<'OPENVPN_UI_RULES'\n*filter\n-F OPENVPN_UI_FORWARD\n-F OPENVPN_UI_INPUT\n")
	for _, rule := range rules {
		b.WriteString("-A OPENVPN_UI_FORWARD -s " + rule.source + "/32 -d " + rule.destination + " -j ACCEPT\n")
		b.WriteString("-A OPENVPN_UI_INPUT -s " + rule.source + "/32 -d " + rule.destination + " -j ACCEPT\n")
	}
	b.WriteString("-A OPENVPN_UI_FORWARD -s " + cidr + " -j DROP\n")
	b.WriteString("-A OPENVPN_UI_INPUT -s " + cidr + " -j DROP\n")
	b.WriteString("COMMIT\nOPENVPN_UI_RULES\n")
	return b.String(), nil
}

func isPrivate(ip net.IP) bool {
	v := ip.To4()
	return v != nil && (v[0] == 10 || (v[0] == 172 && v[1] >= 16 && v[1] <= 31) || (v[0] == 192 && v[1] == 168))
}

func ParseDiscoveredRoutes(raw []byte, vpnCIDR string) ([]DiscoveredRoute, error) {
	var rows []struct {
		Dst     string `json:"dst"`
		Gateway string `json:"gateway"`
		Dev     string `json:"dev"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parse routes: %w", err)
	}
	_, vpn, err := normalizeCIDR(vpnCIDR)
	if err != nil {
		return nil, err
	}
	var result []DiscoveredRoute
	seen := map[string]bool{}
	for _, row := range rows {
		dev := strings.ToLower(row.Dev)
		if row.Dst == "" || row.Dst == "default" || strings.HasPrefix(dev, "docker") || strings.HasPrefix(dev, "br-") || strings.HasPrefix(dev, "veth") || strings.HasPrefix(dev, "tun") || strings.HasPrefix(dev, "tap") || strings.HasPrefix(dev, "wg") || dev == "lo" {
			continue
		}
		cidr, n, err := normalizeCIDR(row.Dst)
		if err != nil {
			continue
		}
		ones, _ := n.Mask.Size()
		if ones >= 32 || !isPrivate(n.IP) || n.Contains(vpn.IP) || vpn.Contains(n.IP) || seen[cidr] {
			continue
		}
		seen[cidr] = true
		result = append(result, DiscoveredRoute{CIDR: cidr, Interface: row.Dev, Gateway: row.Gateway})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CIDR < result[j].CIDR })
	return result, nil
}
