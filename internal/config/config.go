package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config содержит конфигурацию wg-agent
type Config struct {
	// WireGuard настройки
	Interface      string // WireGuard интерфейс по умолчанию (wg0)
	Subnet         string // Подсеть для выделения IP клиентам (10.8.0.0/24)
	ServerPublicIP string // Публичный IP/домен сервера для endpoint
	ServerPort     int    // Порт WireGuard сервера (51820)

	// TLS настройки
	TLSCert  string // Путь к TLS сертификату
	TLSKey   string // Путь к TLS ключу
	CABundle string // Путь к CA bundle для client auth

	// Сетевые настройки
	Addr     string // Адрес для gRPC сервера
	HTTPAddr string // Адрес для health check сервера

	// Лимиты
	RateLimit int // Rate limit для запросов
}

// Load загружает конфигурацию из переменных окружения
func Load() *Config {
	rateLimit := 10
	if rl := os.Getenv("WG_AGENT_RATE_LIMIT"); rl != "" {
		if parsed, err := strconv.Atoi(rl); err == nil {
			rateLimit = parsed
		}
	}

	serverPort := 51820
	if sp := os.Getenv("WG_SERVER_PORT"); sp != "" {
		if parsed, err := strconv.Atoi(sp); err == nil {
			serverPort = parsed
		}
	}

	return &Config{
		// WireGuard
		Interface:      getEnv("WG_AGENT_INTERFACE", "wg0"),
		Subnet:         getEnv("WG_SUBNET", "10.8.0.0/24"),
		ServerPublicIP: getEnv("SERVER_PUBLIC_IP", ""),
		ServerPort:     serverPort,

		// TLS
		TLSCert:  getEnv("WG_AGENT_TLS_CERT", "/etc/wg-agent/cert.pem"),
		TLSKey:   getEnv("WG_AGENT_TLS_PRIVATE", "/etc/wg-agent/key.pem"),
		CABundle: getEnv("WG_AGENT_CA_BUNDLE", "/etc/wg-agent/ca.pem"),

		// Network
		Addr:     getEnv("WG_AGENT_ADDR", "0.0.0.0:7443"),
		HTTPAddr: getEnv("WG_AGENT_HTTP_ADDR", "0.0.0.0:8080"),

		// Limits
		RateLimit: rateLimit,
	}
}

// ServerEndpoint возвращает endpoint сервера в формате "host:port"
func (c *Config) ServerEndpoint() string {
	if c.ServerPublicIP == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", c.ServerPublicIP, c.ServerPort)
}

// Validate проверяет обязательные параметры конфигурации
func (c *Config) Validate() error {
	if c.ServerPublicIP == "" {
		return fmt.Errorf("SERVER_PUBLIC_IP is required")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
