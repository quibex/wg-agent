# WireGuard Agent

🚀 **gRPC service for remote WireGuard management**

A secure, high-performance agent that provides remote WireGuard peer management via gRPC with mTLS authentication.

## Features

- 🔐 **mTLS Authentication** - Mutual TLS with client certificate validation
- ⚡ **Rate Limiting** - Configurable request limiting to protect server resources  
- 🏥 **Health Checks** - HTTP health endpoint for monitoring
- 📊 **Structured Logging** - Built-in structured logging with slog
- 🐳 **Docker Ready** - Production-ready containerization

## Quick Start

### 1. Generate Certificates

```bash
# For development (creates CA + all certificates)
make certs

# For production deployment
scripts/make-server-cert.sh  # On server
scripts/make-client-cert.sh  # For lime-bot
```

### 2. Run with Docker

```bash
docker run -d \
  --name wg-agent \
  --network host \
  --cap-add NET_ADMIN \
  --device /dev/net/tun \
  -v /etc/wg-agent:/etc/wg-agent:ro \
  quibex/wg-agent:latest
```

### 3. Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `WG_AGENT_INTERFACE` | `wg0` | WireGuard interface name |
| `WG_AGENT_ADDR` | `0.0.0.0:7443` | gRPC server address |
| `WG_AGENT_HTTP_ADDR` | `0.0.0.0:8080` | HTTP health server address |
| `WG_AGENT_RATE_LIMIT` | `10` | Requests per second limit |
| `WG_AGENT_TLS_CERT` | `/etc/wg-agent/cert.pem` | Server certificate |
| `WG_AGENT_TLS_KEY` | `/etc/wg-agent/key.pem` | Server private key |
| `WG_AGENT_CA_BUNDLE` | `/etc/wg-agent/ca.pem` | CA certificate |

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
