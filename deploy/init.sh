#!/bin/sh
set -eu
cd /opt/openvpn-ui
OPENVPN_ADMIN_PASSWORD=${OPENVPN_ADMIN_PASSWORD:-}
if [ "${OPENVPN_PUBLIC_HOST:-}" = 'vpn.example.com' ] || [ -z "${OPENVPN_PUBLIC_HOST:-}" ]; then
  echo 'Set OPENVPN_PUBLIC_HOST to your Linux IP or VPN domain in .env.' >&2
  exit 1
fi
if [ -z "${OPENVPN_ADMIN_USERNAME:-}" ] || [ "${#OPENVPN_ADMIN_PASSWORD}" -lt 12 ] || [ "$OPENVPN_ADMIN_PASSWORD" = 'CHANGE_ME_USE_A_LONG_RANDOM_PASSWORD' ]; then
  echo 'Set an administrator username and a unique password (at least 12 characters) in .env.' >&2
  exit 1
fi
mkdir -p db /etc/openvpn/config /etc/openvpn/clients /etc/openvpn/staticclients /etc/openvpn/pki /var/log/openvpn
chmod 700 db /etc/openvpn/clients
./openvpn-ui -config /opt/openvpn-ui/conf -init-only
test -s db/data.db
test -s /etc/openvpn/server.conf
test -s /etc/openvpn/config/client.conf
test -s /etc/openvpn/pki/vars

# Generate in a staging directory: an interrupted generation never replaces an existing CA.
if [ ! -s /etc/openvpn/pki/ca.crt ]; then
  if [ -e /etc/openvpn/pki/private/ca.key ]; then
    echo 'Incomplete existing PKI; restore a consistent backup before continuing.' >&2
    exit 1
  fi
  task_pki=$(mktemp -d)
  trap 'rm -rf "$task_pki"' EXIT
  export EASYRSA_BATCH=1 EASYRSA_PKI="$task_pki/pki"
  cd /usr/share/easy-rsa
  ./easyrsa init-pki
  cp /etc/openvpn/pki/vars "$EASYRSA_PKI/vars"
  ./easyrsa build-ca nopass
  ./easyrsa gen-req server nopass
  ./easyrsa sign-req server server
  ./easyrsa gen-dh
  ./easyrsa gen-crl
  openvpn --genkey secret "$EASYRSA_PKI/ta.key"
  cp -a "$EASYRSA_PKI/." /etc/openvpn/pki/
fi
for file in private/ca.key private/server.key issued/server.crt dh.pem ta.key crl.pem index.txt; do
  test -f "/etc/openvpn/pki/$file" || { echo "PKI file missing: $file; restore a backup." >&2; exit 1; }
done
# OpenVPN drops to nobody and must still read CRL and client-specific configurations.
chmod 755 /etc/openvpn /etc/openvpn/pki /etc/openvpn/staticclients /var/log/openvpn
chmod 644 /etc/openvpn/pki/crl.pem
# The server drops privileges before periodically saving assigned VPN addresses.
touch /etc/openvpn/pki/ipp.txt
chown nobody:nogroup /etc/openvpn/pki/ipp.txt
chmod 600 /etc/openvpn/pki/ipp.txt
cp /etc/openvpn/pki/vars /etc/openvpn/config/easy-rsa.vars
echo 'Initialization complete. Existing database, configuration and CA have been preserved.'
