#!/bin/bash

set -e

CERTS_DIR="/tmp/wg-certs"
CA_KEY="$CERTS_DIR/ca-key.pem"
CA_CERT="$CERTS_DIR/ca.pem"
SERVER_KEY="$CERTS_DIR/server-key.pem"
SERVER_CERT="$CERTS_DIR/server.pem"

echo "Creating temp directory..."
mkdir -p "$CERTS_DIR"

echo "Loading CA from environment..."
if [ -z "$CA_CERT_PEM" ] || [ -z "$CA_KEY_PEM" ]; then
    echo "❌ CA_CERT_PEM and CA_KEY_PEM variables not set"
    exit 1
fi

echo "$CA_CERT_PEM" | base64 -d > "$CA_CERT"
echo "$CA_KEY_PEM" | base64 -d > "$CA_KEY"
chmod 600 "$CA_KEY"
chmod 644 "$CA_CERT"

echo "Generating server certificate for wg-agent..."
openssl genrsa -out "$SERVER_KEY" 4096
openssl req -new -key "$SERVER_KEY" -out "$CERTS_DIR/server.csr" \
    -subj "/C=RU/ST=Moscow/L=Moscow/O=WG-Agent/OU=Server/CN=wg-agent"

# Build SAN dynamically
ALT_NAMES="DNS.1 = wg-agent\nDNS.2 = localhost\nIP.1 = 127.0.0.1\nIP.2 = 0.0.0.0"

if [ -n "$WG_AGENT_SAN_IPS" ]; then
    IFS=',' read -ra _IPS <<< "$WG_AGENT_SAN_IPS"
    idx=3
    for _ip in "${_IPS[@]}"; do
        ALT_NAMES+="\nIP.$idx = ${_ip}"
        idx=$((idx+1))
    done
fi

if [ -n "$WG_AGENT_SAN_DNS" ]; then
    IFS=',' read -ra _DNS <<< "$WG_AGENT_SAN_DNS"
    idx=3
    for _d in "${_DNS[@]}"; do
        ALT_NAMES+="\nDNS.$idx = ${_d}"
        idx=$((idx+1))
    done
fi

cat > "$CERTS_DIR/server.conf" << EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req

[req_distinguished_name]

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
$(echo -e "$ALT_NAMES")
EOF

openssl x509 -req -days 365 -in "$CERTS_DIR/server.csr" -CA "$CA_CERT" -CAkey "$CA_KEY" \
    -CAcreateserial -out "$SERVER_CERT" -extensions v3_req -extfile "$CERTS_DIR/server.conf"

echo "Cleaning temp files..."
rm -f "$CERTS_DIR"/*.csr "$CERTS_DIR"/*.conf "$CERTS_DIR"/*.srl

chmod 600 "$SERVER_KEY"
chmod 644 "$SERVER_CERT" "$CA_CERT"

echo "Installing certificates..."
sudo mkdir -p /etc/wg-agent
sudo cp "$SERVER_CERT" /etc/wg-agent/cert.pem
sudo cp "$SERVER_KEY" /etc/wg-agent/key.pem
sudo cp "$CA_CERT" /etc/wg-agent/ca.pem
sudo chmod 600 /etc/wg-agent/key.pem
sudo chmod 644 /etc/wg-agent/cert.pem /etc/wg-agent/ca.pem

echo "Cleaning temp directory..."
rm -rf "$CERTS_DIR"

echo "✅ Server certificate installed in /etc/wg-agent/" 