#!/bin/sh
set -eu
case "${1:-ui}" in
  init) exec /opt/deploy/init.sh ;;
  server) exec /opt/deploy/server.sh ;;
  ui)
    test -s /etc/openvpn/pki/ca.crt
    exec /opt/openvpn-ui/openvpn-ui -config /opt/openvpn-ui/conf
    ;;
  *) exec "$@" ;;
esac
