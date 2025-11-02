#!/bin/bash

set -e

echo "================================================"
echo "    WG-Agent Server Setup Script"
echo "================================================"
echo ""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR="/opt/wg-agent"
CERTS_DIR="/tmp/wg-certs"
INSTALL_DIR="/etc/wg-agent"
BINARY_PATH="/usr/local/bin/wg-agent"
SERVICE_NAME="wg-agent"
GITHUB_REPO="https://github.com/quibex/wg-agent.git"

check_root() {
    if [[ $EUID -ne 0 ]]; then
        echo "❌ This script must be run as root"
        exit 1
    fi
    echo "✅ Running as root"
}

check_env_file() {
    if [ ! -f ".env" ]; then
        echo "❌ .env file not found in current directory"
        echo "   Please create .env file with required variables"
        echo "   See .env.example for template"
        exit 1
    fi
    echo "✅ .env file found"
    source .env
}

validate_env_vars() {
    local missing_vars=()
    
    [ -z "$CA_CERT_PEM" ] && missing_vars+=("CA_CERT_PEM")
    [ -z "$CA_KEY_PEM" ] && missing_vars+=("CA_KEY_PEM")
    [ -z "$WG_AGENT_INTERFACE" ] && missing_vars+=("WG_AGENT_INTERFACE")
    [ -z "$WG_AGENT_ADDR" ] && missing_vars+=("WG_AGENT_ADDR")
    [ -z "$WG_AGENT_HTTP_ADDR" ] && missing_vars+=("WG_AGENT_HTTP_ADDR")
    [ -z "$WG_AGENT_RATE_LIMIT" ] && missing_vars+=("WG_AGENT_RATE_LIMIT")
    [ -z "$SERVER_PUBLIC_IP" ] && missing_vars+=("SERVER_PUBLIC_IP")
    [ -z "$WG_SERVER_PORT" ] && missing_vars+=("WG_SERVER_PORT")
    [ -z "$WG_SERVER_IP" ] && missing_vars+=("WG_SERVER_IP")
    
    if [ ${#missing_vars[@]} -ne 0 ]; then
        echo "❌ Missing required environment variables:"
        for var in "${missing_vars[@]}"; do
            echo "   - $var"
        done
        exit 1
    fi
    
    echo "✅ All required environment variables are set"
}

install_dependencies() {
    echo ""
    echo "📦 Installing dependencies..."
    
    if command -v apt-get &> /dev/null; then
        apt-get update -qq
        apt-get install -y -qq wireguard wireguard-tools openssl git curl iptables
        
        if ! command -v go &> /dev/null; then
            echo "   Installing Golang..."
            wget -q https://go.dev/dl/go1.23.2.linux-amd64.tar.gz
            rm -rf /usr/local/go
            tar -C /usr/local -xzf go1.23.2.linux-amd64.tar.gz
            rm go1.23.2.linux-amd64.tar.gz
            export PATH=$PATH:/usr/local/go/bin
            echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
        fi
    elif command -v yum &> /dev/null; then
        yum install -y -q wireguard-tools openssl git curl iptables
        
        if ! command -v go &> /dev/null; then
            echo "   Installing Golang..."
            wget -q https://go.dev/dl/go1.23.2.linux-amd64.tar.gz
            rm -rf /usr/local/go
            tar -C /usr/local -xzf go1.23.2.linux-amd64.tar.gz
            rm go1.23.2.linux-amd64.tar.gz
            export PATH=$PATH:/usr/local/go/bin
            echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
        fi
    else
        echo "❌ Unsupported package manager. Install dependencies manually:"
        echo "   - wireguard-tools, openssl, git, curl, golang"
        exit 1
    fi
    
    echo "✅ Dependencies installed"
}

setup_wireguard() {
    echo ""
    echo "🔧 Setting up WireGuard interface..."
    
    if [ -f "/etc/wireguard/${WG_AGENT_INTERFACE}.conf" ]; then
        echo "   ⚠️  WireGuard config already exists, backing up..."
        cp "/etc/wireguard/${WG_AGENT_INTERFACE}.conf" "/etc/wireguard/${WG_AGENT_INTERFACE}.conf.backup.$(date +%s)"
    fi
    
    if [ ! -f "/etc/wireguard/${WG_AGENT_INTERFACE}_private.key" ]; then
        echo "   Generating WireGuard keys..."
        wg genkey | tee "/etc/wireguard/${WG_AGENT_INTERFACE}_private.key" | wg pubkey > "/etc/wireguard/${WG_AGENT_INTERFACE}_public.key"
        chmod 600 "/etc/wireguard/${WG_AGENT_INTERFACE}_private.key"
    fi
    
    local private_key=$(cat "/etc/wireguard/${WG_AGENT_INTERFACE}_private.key")
    local public_key=$(cat "/etc/wireguard/${WG_AGENT_INTERFACE}_public.key")
    
    cat > "/etc/wireguard/${WG_AGENT_INTERFACE}.conf" << EOF
[Interface]
Address = ${WG_SERVER_IP}
ListenPort = ${WG_SERVER_PORT}
PrivateKey = ${private_key}
PostUp = iptables -A FORWARD -i %i -j ACCEPT; iptables -A FORWARD -o %i -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE

EOF
    
    echo "   Enabling IP forwarding..."
    echo "net.ipv4.ip_forward=1" > /etc/sysctl.d/99-wireguard.conf
    sysctl -p /etc/sysctl.d/99-wireguard.conf > /dev/null 2>&1
    
    echo "   Starting WireGuard interface..."
    systemctl enable wg-quick@${WG_AGENT_INTERFACE} > /dev/null 2>&1
    systemctl restart wg-quick@${WG_AGENT_INTERFACE}
    
    echo "✅ WireGuard interface configured"
    echo "   📋 Server Public Key: $public_key"
}

generate_tls_certificates() {
    echo ""
    echo "🔐 Generating TLS certificates..."
    
    mkdir -p "$CERTS_DIR"
    mkdir -p "$INSTALL_DIR"
    
    local CA_KEY="$CERTS_DIR/ca-key.pem"
    local CA_CERT="$CERTS_DIR/ca.pem"
    local SERVER_KEY="$CERTS_DIR/server-key.pem"
    local SERVER_CERT="$CERTS_DIR/server.pem"
    
    echo "   Loading CA from environment..."
    echo "$CA_CERT_PEM" | base64 -d > "$CA_CERT"
    echo "$CA_KEY_PEM" | base64 -d > "$CA_KEY"
    chmod 600 "$CA_KEY"
    chmod 644 "$CA_CERT"
    
    echo "   Generating server certificate..."
    openssl genrsa -out "$SERVER_KEY" 4096 2>/dev/null
    openssl req -new -key "$SERVER_KEY" -out "$CERTS_DIR/server.csr" \
        -subj "/C=RU/ST=Moscow/L=Moscow/O=WG-Agent/OU=Server/CN=wg-agent" 2>/dev/null
    
    local ALT_NAMES="DNS.1 = wg-agent\nDNS.2 = localhost\nIP.1 = 127.0.0.1\nIP.2 = 0.0.0.0\nIP.3 = ${SERVER_PUBLIC_IP}"
    
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
        -CAcreateserial -out "$SERVER_CERT" -extensions v3_req -extfile "$CERTS_DIR/server.conf" 2>/dev/null
    
    echo "   Installing certificates..."
    cp "$SERVER_CERT" "$INSTALL_DIR/cert.pem"
    cp "$SERVER_KEY" "$INSTALL_DIR/key.pem"
    cp "$CA_CERT" "$INSTALL_DIR/ca.pem"
    chmod 600 "$INSTALL_DIR/key.pem"
    chmod 644 "$INSTALL_DIR/cert.pem" "$INSTALL_DIR/ca.pem"
    
    rm -rf "$CERTS_DIR"
    
    echo "✅ TLS certificates installed to $INSTALL_DIR"
}

clone_and_build() {
    echo ""
    echo "🔨 Building wg-agent..."
    
    if [ -d "$WORK_DIR" ]; then
        echo "   Removing old installation..."
        rm -rf "$WORK_DIR"
    fi
    
    mkdir -p "$WORK_DIR"
    cd "$WORK_DIR"
    
    echo "   Cloning repository..."
    if [ -n "$GITHUB_TOKEN" ]; then
        git clone -q "https://${GITHUB_TOKEN}@github.com/YOUR_USERNAME/wg-agent.git" .
    else
        git clone -q "$GITHUB_REPO" .
    fi
    
    echo "   Building binary..."
    export PATH=$PATH:/usr/local/go/bin
    go build -o "$BINARY_PATH" ./cmd/wg-agent 2>/dev/null
    chmod +x "$BINARY_PATH"
    
    echo "✅ wg-agent binary installed to $BINARY_PATH"
}

create_systemd_service() {
    echo ""
    echo "⚙️  Creating systemd service..."
    
    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=WireGuard Agent gRPC Service
After=network.target wg-quick@${WG_AGENT_INTERFACE}.service
Wants=wg-quick@${WG_AGENT_INTERFACE}.service

[Service]
Type=simple
User=root
WorkingDirectory=${WORK_DIR}
ExecStart=${BINARY_PATH}
Restart=always
RestartSec=10
Environment="WG_AGENT_INTERFACE=${WG_AGENT_INTERFACE}"
Environment="WG_AGENT_ADDR=${WG_AGENT_ADDR}"
Environment="WG_AGENT_HTTP_ADDR=${WG_AGENT_HTTP_ADDR}"
Environment="WG_AGENT_RATE_LIMIT=${WG_AGENT_RATE_LIMIT}"
Environment="WG_AGENT_TLS_CERT=${INSTALL_DIR}/cert.pem"
Environment="WG_AGENT_TLS_PRIVATE=${INSTALL_DIR}/key.pem"
Environment="WG_AGENT_CA_BUNDLE=${INSTALL_DIR}/ca.pem"

[Install]
WantedBy=multi-user.target
EOF
    
    systemctl daemon-reload
    systemctl enable ${SERVICE_NAME} > /dev/null 2>&1
    systemctl restart ${SERVICE_NAME}
    
    sleep 2
    
    if systemctl is-active --quiet ${SERVICE_NAME}; then
        echo "✅ wg-agent service started successfully"
    else
        echo "⚠️  Service may have issues. Check logs: journalctl -u ${SERVICE_NAME}"
    fi
}

output_connection_info() {
    echo ""
    echo "================================================"
    echo "    ✅ WG-Agent Setup Complete!"
    echo "================================================"
    echo ""
    echo "📋 Server Information:"
    echo "───────────────────────────────────────────────"
    echo "Server IP:          ${SERVER_PUBLIC_IP}"
    echo "WireGuard Port:     ${WG_SERVER_PORT}"
    echo "WireGuard Public:   $(cat /etc/wireguard/${WG_AGENT_INTERFACE}_public.key)"
    echo ""
    echo "📡 Connection Parameters for Bot:"
    echo "───────────────────────────────────────────────"
    echo "HTTP Endpoint:      http://${SERVER_PUBLIC_IP}:${WG_AGENT_HTTP_ADDR#*:}/health"
    echo "gRPC Endpoint:      ${SERVER_PUBLIC_IP}:${WG_AGENT_ADDR#*:}"
    echo ""
    echo "🔐 TLS Certificates (for bot):"
    echo "───────────────────────────────────────────────"
    echo "CA Certificate:     ${INSTALL_DIR}/ca.pem"
    echo ""
    echo "To copy CA cert to your local machine:"
    echo "  scp root@${SERVER_PUBLIC_IP}:${INSTALL_DIR}/ca.pem ./ca-${SERVER_PUBLIC_IP}.pem"
    echo ""
    echo "Note: Bot should use the same client certificate"
    echo "      that was generated with the CA"
    echo ""
    echo "📊 Service Status:"
    echo "───────────────────────────────────────────────"
    systemctl status ${SERVICE_NAME} --no-pager -l | head -n 10
    echo ""
    echo "📝 Useful Commands:"
    echo "───────────────────────────────────────────────"
    echo "View logs:          journalctl -u ${SERVICE_NAME} -f"
    echo "Restart service:    systemctl restart ${SERVICE_NAME}"
    echo "Check WireGuard:    wg show ${WG_AGENT_INTERFACE}"
    echo "Health check:       curl http://localhost:${WG_AGENT_HTTP_ADDR#*:}/health"
    echo ""
}

main() {
    check_root
    check_env_file
    validate_env_vars
    install_dependencies
    setup_wireguard
    generate_tls_certificates
    clone_and_build
    create_systemd_service
    output_connection_info
}

main "$@"

