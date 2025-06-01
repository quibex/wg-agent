#!/bin/bash

set -e

CERTS_DIR="/tmp/wg-certs"
CA_KEY="$CERTS_DIR/ca-key.pem"
CA_CERT="$CERTS_DIR/ca.pem"
SERVER_KEY="$CERTS_DIR/server-key.pem"
SERVER_CERT="$CERTS_DIR/server.pem"

echo "Создание временной директории..."
mkdir -p "$CERTS_DIR"

echo "Получение CA из переменных окружения..."
if [ -z "$CA_CERT_PEM" ] || [ -z "$CA_KEY_PEM" ]; then
    echo "❌ Переменные CA_CERT_PEM и CA_KEY_PEM не установлены"
    exit 1
fi

echo "$CA_CERT_PEM" | base64 -d > "$CA_CERT"
echo "$CA_KEY_PEM" | base64 -d > "$CA_KEY"
chmod 600 "$CA_KEY"
chmod 644 "$CA_CERT"

echo "Генерация серверного сертификата для wg-agent..."
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

echo "Очистка временных файлов..."
rm -f "$CERTS_DIR"/*.csr "$CERTS_DIR"/*.conf "$CERTS_DIR"/*.srl

chmod 600 "$SERVER_KEY"
chmod 644 "$SERVER_CERT" "$CA_CERT"

echo "Установка сертификатов..."
sudo mkdir -p /etc/wg-agent
sudo cp "$SERVER_CERT" /etc/wg-agent/cert.pem
sudo cp "$SERVER_KEY" /etc/wg-agent/key.pem
sudo cp "$CA_CERT" /etc/wg-agent/ca.pem
sudo chmod 600 /etc/wg-agent/key.pem
sudo chmod 644 /etc/wg-agent/cert.pem /etc/wg-agent/ca.pem

echo "Очистка временной директории..."
rm -rf "$CERTS_DIR"

echo "✅ Серверный сертификат установлен в /etc/wg-agent/" 