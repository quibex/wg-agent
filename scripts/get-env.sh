#!/bin/bash

set -e

CERTS_DIR="certs"
CA_CERT="$CERTS_DIR/ca.pem"
CA_KEY="$CERTS_DIR/ca-key.pem"

if [ ! -f "$CA_CERT" ] || [ ! -f "$CA_KEY" ]; then
    echo "❌ CA certificates not found in $CERTS_DIR/"
    echo ""
    echo "Run this first:"
    echo "  ./scripts/make-ca-only.sh"
    echo ""
    exit 1
fi

echo "================================================"
echo "📋 Copy this to .env file on your server:"
echo "================================================"
echo ""

CA_CERT_B64=$(cat "$CA_CERT" | base64 | tr -d '\n')
CA_KEY_B64=$(cat "$CA_KEY" | base64 | tr -d '\n')

cat << ENVEOF
CA_CERT_PEM=$CA_CERT_B64
CA_KEY_PEM=$CA_KEY_B64
SERVER_PUBLIC_IP=your.server.ip.address
ENVEOF

echo ""
echo "================================================"
echo ""
echo "💡 Replace 'your.server.ip.address' with actual server IP"
echo ""


