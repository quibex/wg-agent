#!/bin/bash

set -e

CERTS_DIR="certs"
CA_KEY="$CERTS_DIR/ca-key.pem"
CA_CERT="$CERTS_DIR/ca.pem"

echo "================================================"
echo "    CA Certificate Generator"
echo "================================================"
echo ""
echo "This script generates a Certificate Authority (CA)"
echo "that will be used to sign all server certificates."
echo ""

if [ -f "$CA_CERT" ] || [ -f "$CA_KEY" ]; then
    echo "⚠️  CA certificate already exists!"
    echo ""
    ls -lh "$CA_CERT" "$CA_KEY" 2>/dev/null || true
    echo ""
    read -p "Do you want to regenerate? This will invalidate all existing server certificates! (yes/no): " confirm
    
    if [ "$confirm" != "yes" ]; then
        echo "Aborted."
        exit 0
    fi
    
    echo "Backing up existing CA..."
    [ -f "$CA_CERT" ] && mv "$CA_CERT" "$CA_CERT.backup.$(date +%s)"
    [ -f "$CA_KEY" ] && mv "$CA_KEY" "$CA_KEY.backup.$(date +%s)"
fi

echo "Creating certificates directory..."
mkdir -p "$CERTS_DIR"

echo "Generating CA private key..."
openssl genrsa -out "$CA_KEY" 4096

echo "Generating CA certificate..."
openssl req -new -x509 -days 3650 -key "$CA_KEY" -out "$CA_CERT" \
    -subj "/C=RU/ST=Moscow/L=Moscow/O=WG-Agent/OU=CA/CN=wg-agent-ca"

echo "Setting permissions..."
chmod 600 "$CA_KEY"
chmod 644 "$CA_CERT"

echo ""
echo "✅ CA certificate generated successfully!"
echo ""
echo "📁 Files created:"
echo "   - $CA_CERT (public certificate)"
echo "   - $CA_KEY (private key - KEEP SECURE!)"
echo ""
echo "📋 Next steps:"
echo ""
echo "1. Encode certificates for .env file:"
echo "   CA_CERT_PEM=\$(cat $CA_CERT | base64 | tr -d '\n')"
echo "   CA_KEY_PEM=\$(cat $CA_KEY | base64 | tr -d '\n')"
echo ""
echo "2. Add these to your .env file on each server"
echo ""
echo "3. Generate client certificate for bot:"
echo "   ./scripts/make-client-cert.sh"
echo ""
echo "⚠️  IMPORTANT: Keep the CA private key ($CA_KEY) secure!"
echo "   This key will be used to generate all server certificates."
echo ""
