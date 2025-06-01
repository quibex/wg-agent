#!/bin/bash

set -e

echo "🚀 Настройка dev окружения для wg-agent..."

echo "1️⃣ Проверка зависимостей..."
if ! command -v openssl &> /dev/null; then
    echo "❌ OpenSSL не найден. Установите: brew install openssl (macOS) или apt install openssl (Linux)"
    exit 1
fi

if ! command -v protoc &> /dev/null; then
    echo "❌ Protoc не найден. Установите: brew install protobuf (macOS) или apt install protobuf-compiler (Linux)"
    exit 1
fi

echo "2️⃣ Установка Go зависимостей..."
go mod tidy
go mod download

echo "3️⃣ Генерация protobuf файлов..."
make proto

echo "4️⃣ Создание TLS сертификатов..."
make certs

echo "5️⃣ Компиляция приложения..."
make build

echo ""
echo "✅ Dev окружение готово!"
echo ""
echo "Для запуска:"
echo "  make run-agent     # Запустить с dev сертификатами"
echo ""
echo "Для тестирования health check:"
echo "  curl http://localhost:8080/health"
echo ""
echo "Сертификаты созданы в директории certs/:"
echo "  📁 CA:      certs/ca.pem"
echo "  🖥️ Server:  certs/server.pem"
echo "  🤖 Client:  certs/client.pem (для bot-service)" 