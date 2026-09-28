#!/bin/sh
set -eu
if [ "$(cat /proc/sys/net/ipv4/ip_forward)" != "1" ]; then
  echo "net.ipv4.ip_forward must be enabled on the Linux host" >&2
  exit 1
fi
# Read the persisted configuration so a later UI change also updates NAT on restart.
vpn_address=$(awk '$1 == "server" { print $2; exit }' /etc/openvpn/server.conf)
vpn_netmask=$(awk '$1 == "server" { print $3; exit }' /etc/openvpn/server.conf)
vpn_prefix=$(ipcalc -p "$vpn_address" "$vpn_netmask" | awk -F= '$1 == "PREFIX" { print $2 }')
vpn_network="$vpn_address/$vpn_prefix"
test -n "$vpn_address" -a -n "$vpn_prefix"
# Install a fail-closed baseline before OpenVPN starts. The UI replaces the
# contents of these chains after loading group policy from the database.
iptables -N OPENVPN_UI_FORWARD 2>/dev/null || true
iptables -N OPENVPN_UI_INPUT 2>/dev/null || true
iptables -C FORWARD -s "$vpn_network" -j OPENVPN_UI_FORWARD 2>/dev/null ||
  iptables -I FORWARD 1 -s "$vpn_network" -j OPENVPN_UI_FORWARD
iptables -C INPUT -s "$vpn_network" -j OPENVPN_UI_INPUT 2>/dev/null ||
  iptables -I INPUT 1 -s "$vpn_network" -j OPENVPN_UI_INPUT
iptables-restore --noflush <<EOF
*filter
-F OPENVPN_UI_FORWARD
-F OPENVPN_UI_INPUT
-A OPENVPN_UI_FORWARD -s $vpn_network -j DROP
-A OPENVPN_UI_INPUT -s $vpn_network -j DROP
COMMIT
EOF
if [ -n "${OPENVPN_NAT_INTERFACE:-}" ]; then
  iptables -t nat -C POSTROUTING -s "$vpn_network" -o "$OPENVPN_NAT_INTERFACE" -j MASQUERADE 2>/dev/null ||
    iptables -t nat -A POSTROUTING -s "$vpn_network" -o "$OPENVPN_NAT_INTERFACE" -j MASQUERADE
else
  iptables -t nat -C POSTROUTING -s "$vpn_network" -j MASQUERADE 2>/dev/null ||
    iptables -t nat -A POSTROUTING -s "$vpn_network" -j MASQUERADE
fi
iptables -C FORWARD -o tun0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT 2>/dev/null ||
  iptables -A FORWARD -o tun0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
exec openvpn --cd /etc/openvpn --config /etc/openvpn/server.conf
