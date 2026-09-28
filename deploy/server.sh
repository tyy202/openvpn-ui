#!/bin/sh
set -eu
# Read the persisted configuration so a later UI change also updates NAT on restart.
vpn_network=$(awk '$1 == "server" { print $2 "/" $3; exit }' /etc/openvpn/server.conf)
test -n "$vpn_network"
iptables -t nat -C POSTROUTING -s "$vpn_network" -o eth0 -j MASQUERADE 2>/dev/null ||
  iptables -t nat -A POSTROUTING -s "$vpn_network" -o eth0 -j MASQUERADE
iptables -C FORWARD -i tun0 -j ACCEPT 2>/dev/null || iptables -A FORWARD -i tun0 -j ACCEPT
iptables -C FORWARD -o tun0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT 2>/dev/null ||
  iptables -A FORWARD -o tun0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
exec openvpn --cd /etc/openvpn --config /etc/openvpn/server.conf
