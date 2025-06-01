#!/bin/bash

set -e

CERTS_DIR="certs"
CA_KEY="$CERTS_DIR/ca-key.pem"
CA_CERT="$CERTS_DIR/ca.pem"
SERVER_KEY="$CERTS_DIR/server-key.pem"
SERVER_CERT="$CERTS_DIR/server.pem"
CLIENT_KEY="$CERTS_DIR/lime-bot-key.pem"
CLIENT_CERT="$CERTS_DIR/lime-bot.pem"

echo "Creating certificates directory..."
mkdir -p "$CERTS_DIR"

echo "Generating CA..."
openssl genrsa -out "$CA_KEY" 4096
openssl req -new -x509 -days 3650 -key "$CA_KEY" -out "$CA_CERT" \
    -subj "/C=RU/ST=Moscow/L=Moscow/O=WG-Project/OU=CA/CN=wg-project-ca"

echo "Generating server certificate for wg-agent..."
openssl genrsa -out "$SERVER_KEY" 4096
openssl req -new -key "$SERVER_KEY" -out "$CERTS_DIR/server.csr" \
    -subj "/C=RU/ST=Moscow/L=Moscow/O=WG-Project/OU=Server/CN=wg-agent"

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
DNS.1 = wg-agent
DNS.2 = localhost
IP.1 = 127.0.0.1
IP.2 = 0.0.0.0
EOF

openssl x509 -req -days 365 -in "$CERTS_DIR/server.csr" -CA "$CA_CERT" -CAkey "$CA_KEY" \
    -CAcreateserial -out "$SERVER_CERT" -extensions v3_req -extfile "$CERTS_DIR/server.conf"

echo "Generating client certificate for lime-bot..."
openssl genrsa -out "$CLIENT_KEY" 4096
openssl req -new -key "$CLIENT_KEY" -out "$CERTS_DIR/client.csr" \
    -subj "/C=RU/ST=Moscow/L=Moscow/O=WG-Project/OU=Client/CN=lime-bot"

openssl x509 -req -days 365 -in "$CERTS_DIR/client.csr" -CA "$CA_CERT" -CAkey "$CA_KEY" \
    -CAcreateserial -out "$CLIENT_CERT"

echo "Cleaning up temporary files..."
rm -f "$CERTS_DIR"/*.csr "$CERTS_DIR"/*.conf "$CERTS_DIR"/*.srl

echo "Setting permissions..."
chmod 600 "$CERTS_DIR"/*-key.pem
chmod 644 "$CERTS_DIR"/*.pem

echo ""
echo "✅ Certificates created in $CERTS_DIR:"
echo "  📁 CA:       $CA_CERT, $CA_KEY"
echo "  🖥️  Server:   $SERVER_CERT, $SERVER_KEY" 
echo "  🤖 lime-bot: $CLIENT_CERT, $CLIENT_KEY"
echo ""
echo "For wg-agent deployment:"
echo "  sudo mkdir -p /etc/wg-agent"
echo "  sudo cp $CERTS_DIR/server.pem /etc/wg-agent/cert.pem"
echo "  sudo cp $CERTS_DIR/server-key.pem /etc/wg-agent/key.pem"
echo "  sudo cp $CERTS_DIR/ca.pem /etc/wg-agent/ca.pem"
echo ""
echo "For lime-bot deployment:"
echo "  sudo mkdir -p /etc/lime-bot"
echo "  sudo cp $CERTS_DIR/lime-bot.pem /etc/lime-bot/client.pem"
echo "  sudo cp $CERTS_DIR/lime-bot-key.pem /etc/lime-bot/client-key.pem"
echo "  sudo cp $CERTS_DIR/ca.pem /etc/lime-bot/ca.pem" 