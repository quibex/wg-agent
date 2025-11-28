#!/bin/bash

set -e

CERTS_DIR="certs"
CA_KEY="$CERTS_DIR/ca-key.pem"
CA_CERT="$CERTS_DIR/ca.pem"
CLIENT_KEY="$CERTS_DIR/client-key.pem"
CLIENT_CERT="$CERTS_DIR/client.pem"

echo "================================================"
echo "    Client Certificate Generator"
echo "================================================"
echo ""

if [ ! -f "$CA_CERT" ] || [ ! -f "$CA_KEY" ]; then
    echo "❌ CA certificate not found!"
    echo "   Please run ./scripts/make-ca-only.sh first"
    exit 1
fi

if [ -f "$CLIENT_CERT" ] || [ -f "$CLIENT_KEY" ]; then
    echo "⚠️  Client certificate already exists!"
    echo ""
    ls -lh "$CLIENT_CERT" "$CLIENT_KEY" 2>/dev/null || true
    echo ""
    read -p "Do you want to regenerate? (yes/no): " confirm
    
    if [ "$confirm" != "yes" ]; then
        echo "Aborted."
        exit 0
    fi
fi

echo "Generating client private key..."
openssl genrsa -out "$CLIENT_KEY" 4096

echo "Generating client certificate signing request..."
openssl req -new -key "$CLIENT_KEY" -out "$CERTS_DIR/client.csr" \
    -subj "/C=RU/ST=Moscow/L=Moscow/O=WG-Agent/OU=Client/CN=kurut-bot"

echo "Signing client certificate with CA..."
openssl x509 -req -days 365 -in "$CERTS_DIR/client.csr" \
    -CA "$CA_CERT" -CAkey "$CA_KEY" -CAcreateserial \
    -out "$CLIENT_CERT"

echo "Cleaning up temporary files..."
rm -f "$CERTS_DIR"/*.csr "$CERTS_DIR"/*.srl

echo "Setting permissions..."
chmod 600 "$CLIENT_KEY"
chmod 644 "$CLIENT_CERT"

echo ""
echo "✅ Client certificate generated successfully!"
echo ""
echo "📁 Files created:"
echo "   - $CLIENT_CERT (client certificate)"
echo "   - $CLIENT_KEY (client private key)"
echo ""
echo "================================================"
echo "    Environment Variables for kurut-bot"
echo "================================================"
echo ""
echo "Add these to your .env file or environment:"
echo ""
echo "# WireGuard TLS Configuration"
echo "WIREGUARD_TLS_CA_CERT=$(base64 -i "$CA_CERT" | tr -d '\n')"
echo "WIREGUARD_TLS_CLIENT_CERT=$(base64 -i "$CLIENT_CERT" | tr -d '\n')"
echo "WIREGUARD_TLS_CLIENT_KEY=$(base64 -i "$CLIENT_KEY" | tr -d '\n')"
echo "WIREGUARD_TLS_SERVER_NAME=wg-agent"
echo ""
echo "================================================"
echo ""
echo "📝 Note: Certificates are base64 encoded for easy ENV usage"
echo "   No need to copy files - just set ENV variables!"
echo ""

