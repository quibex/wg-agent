# WireGuard Agent

🚀 **gRPC service for remote WireGuard management**

A secure, high-performance agent that provides remote WireGuard peer management via gRPC with mTLS authentication.

## Features

- 🔐 **mTLS Authentication** - Mutual TLS with client certificate validation
- ⚡ **Rate Limiting** - Configurable request limiting to protect server resources  
- 🏥 **Health Checks** - HTTP health endpoint for monitoring
- 📊 **Structured Logging** - Built-in structured logging with slog
- 🚀 **Automated Setup** - One-script deployment on new servers

## Quick Start - Production Deployment

### 1. Generate CA Certificate (One Time Only)

On your local machine, generate the Certificate Authority that will sign all server certificates:

```bash
cd wg-agent
chmod +x scripts/make-ca-only.sh
./scripts/make-ca-only.sh
```

This creates `certs/ca.pem` and `certs/ca-key.pem`. Keep these secure!

### 2. Generate Client Certificate for Bot (One Time Only)

```bash
chmod +x scripts/make-client-cert.sh
./scripts/make-client-cert.sh
```

This creates client certificates in `certs/` that your bot will use to connect to all wg-agent servers.

### 3. Deploy to New Server

On your new VPS server:

```bash
# 1. Download the setup script
wget https://raw.githubusercontent.com/YOUR_USERNAME/wg-agent/main/scripts/setup-server.sh
chmod +x setup-server.sh

# 2. Create .env file with your configuration
cat > .env << 'EOF'
CA_CERT_PEM=$(cat certs/ca.pem | base64 | tr -d '\n')
CA_KEY_PEM=$(cat certs/ca-key.pem | base64 | tr -d '\n')
WG_AGENT_INTERFACE=wg0
WG_AGENT_ADDR=0.0.0.0:7443
WG_AGENT_HTTP_ADDR=0.0.0.0:8080
WG_AGENT_RATE_LIMIT=10
SERVER_PUBLIC_IP=YOUR_SERVER_IP
WG_SERVER_PORT=51820
WG_SERVER_IP=10.8.0.1/24
EOF

# 3. Run the setup script
sudo ./setup-server.sh
```

The script will:
- Install dependencies (Go, WireGuard, OpenSSL)
- Configure WireGuard interface
- Generate unique TLS certificates for this server
- Build and install wg-agent as a systemd service
- Display connection parameters for your bot

### 4. Add Server to Bot

After setup completes, copy the displayed connection parameters and add the server to your bot using the admin interface.

## Configuration

### Environment Variables

See `.env.example` for all available options. Key variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `WG_AGENT_INTERFACE` | `wg0` | WireGuard interface name |
| `WG_AGENT_ADDR` | `0.0.0.0:7443` | gRPC server address |
| `WG_AGENT_HTTP_ADDR` | `0.0.0.0:8080` | HTTP health server address |
| `WG_AGENT_RATE_LIMIT` | `10` | Requests per second limit |
| `SERVER_PUBLIC_IP` | - | Public IP of the server (required) |
| `WG_SERVER_PORT` | `51820` | WireGuard listen port |
| `WG_SERVER_IP` | - | VPN IP range in CIDR notation (required) |

### Rate Limiting

Recommended limits based on server capacity:

- **1-2 cores**: 3 RPS
- **2-4 cores**: 8 RPS  
- **4+ cores**: 15 RPS

## Development

### Local Setup

```bash
# Generate certificates for development
make certs

# Build
make build

# Run tests
make test

# Run locally
make run-agent
```

## API

### gRPC Methods

#### Peer Management

- `AddPeer(interface, publicKey, allowedIP, keepalive, peerID)` - Add WireGuard peer and get configuration
  - Returns: server port, client configuration, QR code
- `RemovePeer(interface, publicKey)` - Remove WireGuard peer completely
- `DisablePeer(interface, publicKey)` - Temporarily disable peer (block traffic)
- `EnablePeer(interface, publicKey)` - Enable previously disabled peer

#### Monitoring & Information

- `GetPeerInfo(interface, publicKey)` - Get detailed peer information
  - Returns: public key, IP, last handshake, RX/TX traffic, status, peer ID
- `ListPeers(interface)` - List all peers with basic information

#### Configuration Generation

- `GeneratePeerConfig(interface, serverEndpoint, dnsServers, allowedIPs)` - Generate new key pair and configuration
  - Returns: private/public keys, configuration, QR code, allocated IP

### Health Check

- `GET /health` → `200 OK` (HTTP endpoint on port 8080)

## Rate Limiting

Recommended limits based on server capacity:

- **1-2 cores**: 3 RPS
- **2-4 cores**: 8 RPS  
- **4+ cores**: 15 RPS

## Security

- **mTLS** with client certificate validation
- **TLS 1.3** minimum version
- **Isolated containers** with minimal privileges
- **Non-root execution** in Docker

## Development

```bash
# Build
make build

# Run tests
make test

# Generate certificates for local development  
make certs

# Run locally
./wg-agent
```

## License

MIT License
