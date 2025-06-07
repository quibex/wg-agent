# GitHub Secrets Configuration

Список обязательных секретов для работы CI/CD пайплайна:

## Docker Hub секреты

| Секрет | Описание | Пример значения |
|--------|----------|-----------------|
| `DOCKERHUB_USERNAME` | Имя пользователя Docker Hub | `your-username` |
| `DOCKERHUB_TOKEN` | Токен доступа Docker Hub | `dckr_pat_...` |

## SSH для деплоя

| Секрет | Описание | Пример значения |
|--------|----------|-----------------|
| `SSH_DEPLOY_KEY` | Приватный SSH ключ для доступа к серверу | `-----BEGIN OPENSSH PRIVATE KEY-----...` |
| `SSH_HOST` | IP адрес или домен сервера | `1.2.3.4` или `server.example.com` |
| `SSH_USER` | Пользователь для SSH подключения | `root` или `ubuntu` |

## TLS сертификаты (Certificate Authority)

| Секрет | Описание | Пример значения |
|--------|----------|-----------------|
| `CA_CERT_PEM` | Сертификат CA в формате PEM | `-----BEGIN CERTIFICATE-----...` |
| `CA_KEY_PEM` | Приватный ключ CA в формате PEM | `-----BEGIN PRIVATE KEY-----...` |

## Telegram уведомления (опционально)

| Секрет | Описание | Пример значения |
|--------|----------|-----------------|
| `TG_TOKEN` | Токен Telegram бота для уведомлений | `1234567890:AAAA...` |
| `TG_CHAT_ID` | Твой Telegram ID для получения уведомлений | `123456789` |

## Как получить секреты

### Docker Hub токен

1. Зайти в Docker Hub → Account Settings → Security
2. Создать новый Access Token
3. Скопировать токен

### SSH ключ

```bash
# Сгенерировать новую пару ключей
ssh-keygen -t ed25519 -C "deploy@github-actions"

# Добавить публичный ключ на сервер
ssh-copy-id -i ~/.ssh/id_ed25519.pub user@server

# Скопировать приватный ключ в GitHub Secrets
cat ~/.ssh/id_ed25519
```

### CA сертификаты

Используйте сертификаты созданные через `make certs` или созданные вручную:

```bash
# Создать CA
openssl req -x509 -newkey rsa:4096 -keyout ca-key.pem -out ca.pem -days 365 -nodes

# Скопировать содержимое файлов в GitHub Secrets
cat ca.pem      # для CA_CERT_PEM
cat ca-key.pem  # для CA_KEY_PEM
```

### Telegram бот

1. **Создать бота через @BotFather** в Telegram:
   - Отправить команду `/newbot`
   - Выбрать имя и username для бота
   - Сохранить полученный токен (например: `1234567890:AAAA...`)

2. **Получить свой Telegram ID**:
   - Написать сообщение боту @userinfobot
   - Он пришлет твой ID (например: `123456789`)

   Альтернативный способ:
   - Написать `/start` своему созданному боту
   - Перейти по ссылке: `https://api.telegram.org/bot<твой_токен>/getUpdates`
   - Найти твой `id` в ответе

## Проверка конфигурации

Все секреты должны быть настроены в:

- GitHub Repository → Settings → Secrets and variables → Actions

После настройки секретов можно запускать:

- Автоматический деплой при push в main
- Ручной деплой скрипта через "Deploy Health Checker Script" workflow
