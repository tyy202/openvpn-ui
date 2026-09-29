package networkscope

import (
	"fmt"
	"net"
	"strings"
)

// Scope is the set of network CIDRs a Web login may manage.
type Scope struct {
	All   bool
	CIDRs map[string]struct{}
}

func (s Scope) Allows(cidr string) bool {
	if s.All {
		return true
	}
	_, ok := s.CIDRs[normalizeCIDR(cidr)]
	return ok
}

// Resolve parses OPENVPN_UI_NETWORK_SCOPES and resolves a login's scope.
// With no configuration, legacy behavior is preserved: administrators have
// full access and non-administrators have no access to the policy console.
func Resolve(raw, login string, isAdmin bool) (Scope, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Scope{All: isAdmin}, isAdmin, nil
	}
	raw = strings.TrimRight(raw, "; \t\r\n")
	if raw == "" {
		return Scope{}, false, fmt.Errorf("network scope configuration contains no entries")
	}

	scopes := make(map[string]Scope)
	for _, entry := range strings.Split(raw, ";") {
		entry = strings.TrimSpace(entry)
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return Scope{}, false, fmt.Errorf("invalid network scope entry %q", entry)
		}
		username := strings.ToLower(strings.TrimSpace(parts[0]))
		if _, exists := scopes[username]; exists {
			return Scope{}, false, fmt.Errorf("duplicate network scope login %q", parts[0])
		}

		values := strings.Split(parts[1], ",")
		if len(values) == 1 && strings.TrimSpace(values[0]) == "*" {
			scopes[username] = Scope{All: true}
			continue
		}

		scope := Scope{CIDRs: make(map[string]struct{}, len(values))}
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "*" {
				return Scope{}, false, fmt.Errorf("wildcard must be the only scope for login %q", parts[0])
			}
			cidr, err := parseCIDR(value)
			if err != nil {
				return Scope{}, false, fmt.Errorf("invalid network scope for login %q: %w", parts[0], err)
			}
			scope.CIDRs[cidr] = struct{}{}
		}
		scopes[username] = scope
	}

	scope, ok := scopes[strings.ToLower(strings.TrimSpace(login))]
	return scope, ok, nil
}

func parseCIDR(value string) (string, error) {
	ip, network, err := net.ParseCIDR(value)
	if err != nil || ip.To4() == nil {
		return "", fmt.Errorf("expected an IPv4 network in CIDR notation")
	}
	return network.String(), nil
}

func normalizeCIDR(value string) string {
	_, network, err := net.ParseCIDR(strings.TrimSpace(value))
	if err != nil {
		return strings.TrimSpace(value)
	}
	return network.String()
}
