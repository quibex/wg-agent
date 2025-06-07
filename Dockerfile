FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o wg-agent ./cmd/wg-agent

FROM alpine:latest

RUN apk --no-cache add ca-certificates

RUN addgroup -g 1001 wgagent && \
    adduser -D -s /bin/sh -u 1001 -G wgagent wgagent

WORKDIR /root/

COPY --from=builder /app/wg-agent .

RUN mkdir -p /etc/wg-agent

# Application configuration
ENV WG_AGENT_INTERFACE=wg0 \
    WG_AGENT_TLS_CERT=/etc/wg-agent/cert.pem \
    WG_AGENT_TLS_PRIVATE=/etc/wg-agent/key.pem \
    WG_AGENT_CA_BUNDLE=/etc/wg-agent/ca.pem \
    WG_AGENT_ADDR=0.0.0.0:7443 \
    WG_AGENT_HTTP_ADDR=0.0.0.0:8080 \
    WG_AGENT_RATE_LIMIT=10

EXPOSE 7443 8080

CMD ["./wg-agent"] 