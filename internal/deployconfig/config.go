// Package deployconfig validates optional first-run Docker configuration.
package deployconfig

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
)

type Config struct {
	Host, Port, Server, PushRoute string
}

var hostname = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?$`)

// Read returns an inactive config when no deployment host is supplied, preserving
// the upstream defaults for non-Docker installations.
func Read(getenv func(string) string) (Config, error) {
	c := Config{Host: getenv("OPENVPN_PUBLIC_HOST")}
	if c.Host == "" {
		return c, nil
	}
	if !hostname.MatchString(c.Host) || len(c.Host) > 253 {
		return c, fmt.Errorf("OPENVPN_PUBLIC_HOST must be an IPv4 address or DNS name without a protocol or port")
	}
	c.Port = getenv("OPENVPN_PUBLIC_PORT")
	if c.Port == "" {
		c.Port = "1194"
	}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return c, fmt.Errorf("OPENVPN_PUBLIC_PORT must be between 1 and 65535")
	}
	vpn := getenv("OPENVPN_VPN_CIDR")
	if vpn == "" {
		vpn = "10.8.0.0/24"
	}
	lan := getenv("OPENVPN_LAN_CIDR")
	if lan == "" {
		lan = "192.168.18.0/24"
	}
	vpnNet, err := network(vpn)
	if err != nil {
		return c, fmt.Errorf("OPENVPN_VPN_CIDR: %w", err)
	}
	bits, _ := vpnNet.Mask.Size()
	if bits != 24 {
		return c, fmt.Errorf("OPENVPN_VPN_CIDR must use /24 (required by client static-IP generation)")
	}
	lanNet, err := network(lan)
	if err != nil {
		return c, fmt.Errorf("OPENVPN_LAN_CIDR: %w", err)
	}
	if vpnNet.Contains(lanNet.IP) || lanNet.Contains(vpnNet.IP) {
		return c, fmt.Errorf("VPN and LAN subnets must not overlap")
	}
	c.Server = fmt.Sprintf("server %s %s", vpnNet.IP, net.IP(vpnNet.Mask))
	c.PushRoute = fmt.Sprintf("push \"route %s %s\"", lanNet.IP, net.IP(lanNet.Mask))
	return c, nil
}

func network(value string) (*net.IPNet, error) {
	ip, subnet, err := net.ParseCIDR(value)
	if err != nil || ip.To4() == nil || !ip.Equal(subnet.IP) {
		return nil, fmt.Errorf("expected an IPv4 network address in CIDR notation")
	}
	return subnet, nil
}
