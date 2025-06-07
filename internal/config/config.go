package config

import (
	"os"
	"strconv"
)

// Config содержит конфигурацию wg-agent
type Config struct {
	Interface string // WireGuard интерфейс по умолчанию
	TLSCert   string // Путь к TLS сертификату
	TLSKey    string // Путь к TLS ключу
	CABundle  string // Путь к CA bundle для client auth
	Addr      string // Адрес для bind
	HTTPAddr  string // Адрес для health check сервера
	RateLimit int    // Rate limit for the agent
}

// Load загружает конфигурацию из переменных окружения
func Load() *Config {
	rateLimit := 10
	if rl := os.Getenv("WG_AGENT_RATE_LIMIT"); rl != "" {
		if parsed, err := strconv.Atoi(rl); err == nil {
			rateLimit = parsed
		}
	}

	return &Config{
		Interface: getEnv("WG_AGENT_INTERFACE", "wg0"),
		TLSCert:   getEnv("WG_AGENT_TLS_CERT", "/etc/wg-agent/cert.pem"),
		TLSKey:    getEnv("WG_AGENT_TLS_PRIVATE", "/etc/wg-agent/key.pem"),
		CABundle:  getEnv("WG_AGENT_CA_BUNDLE", "/etc/wg-agent/ca.pem"),
		Addr:      getEnv("WG_AGENT_ADDR", "0.0.0.0:7443"),
		HTTPAddr:  getEnv("WG_AGENT_HTTP_ADDR", "0.0.0.0:8080"),
		RateLimit: rateLimit,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
