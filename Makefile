.PHONY: proto build run-agent test clean dev certs

# Генерация protobuf файлов
proto:
	@echo "Генерация protobuf файлов..."
	@mkdir -p api/proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/agent.proto

# Сборка wg-agent
build:
	@echo "Сборка wg-agent..."
	go build -o bin/wg-agent ./cmd/wg-agent

# Генерация TLS сертификатов
certs:
	@echo "Генерация TLS сертификатов..."
	chmod +x scripts/make-ca.sh
	./scripts/make-ca.sh

# Запуск wg-agent в dev режиме
run-agent: certs
	@echo "Запуск wg-agent..."
	@if [ ! -f certs/server.pem ]; then \
		echo "Сертификаты не найдены, создаем..."; \
		make certs; \
	fi
	WG_AGENT_TLS_CERT=certs/server.pem \
	WG_AGENT_TLS_KEY=certs/server-key.pem \
	WG_AGENT_CA_BUNDLE=certs/ca.pem \
	go run ./cmd/wg-agent

# Запуск тестов
test:
	go test -v ./...

# Очистка
clean:
	rm -rf bin/
	rm -rf api/proto/
	rm -rf certs/

# Настройка dev окружения
dev: proto certs
	@echo "Настройка dev окружения завершена"

# Установка зависимостей
deps:
	go mod tidy
	go mod download 