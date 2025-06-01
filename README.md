# Техническое задание: система продажи доступа к WireGuard

---

## 0. Общая архитектура

```
┌───────────────┐          mTLS/gRPC          ┌────────────────┐
│ Telegram BOT  │  ◀──────────────────────── ▶│ wg‑agent #1    │  (WG‑server‑1)
│ (bot‑service) │                             └────────────────┘
│ + Postgres    │                             ┌────────────────┐
└───────────────┘  ◀──────────────────────── ▶│ wg‑agent #N    │  (WG‑server‑N)
```

* **bot‑service**‑— один (масштабируется горизонтально);
* **wg‑agent**‑— лёгкий демон, по одному на каждый хост с WireGuard.

---

## 1. Сервис №1: **wg‑agent**

### 1.1 Назначение

* Принимает команды от bot‑service и выполняет операции над локальным интерфейсом WireGuard **без перезапуска интерфейса**.
* Не хранит бизнес‑логику и БД; только thin‑proxy к UAPI.

### 1.2 Технологии

* **Go 1.22**
* gRPC + Protocol Buffers v3
* Библиотека `wgctrl` (`golang.zx2c4.com/wireguard/wgctrl`)

### 1.3 Интерфейс (proto)

Файл `api/agent.proto`:

```proto
syntax = "proto3";
package wgagent;
import "google/protobuf/empty.proto";

service WireGuardAgent {
  rpc AddPeer(AddPeerRequest) returns (AddPeerResponse);
  rpc RemovePeer(RemovePeerRequest) returns (google.protobuf.Empty);
  rpc ListPeers(ListPeersRequest)   returns (ListPeersResponse);  // debug/help
}
message AddPeerRequest  {
  string interface   = 1;  // "wg0"
  string public_key  = 2;
  string allowed_ip  = 3;  // "10.8.0.10/32"
  int32  keepalive_s = 4;  // 25
}
message AddPeerResponse { int32 listen_port = 1; }
message RemovePeerRequest { string interface = 1; string public_key = 2; }
message ListPeersRequest  { string interface = 1; }
message ListPeersResponse { repeated string public_keys = 1; }
```

### 1.4 Функциональные требования

| # | Требование    | Подробности                                                                                                       |
| - | ------------- | ----------------------------------------------------------------------------------------------------------------- |
| 1 | Добавить пира | `AddPeer` должен: 1) валидировать вход, 2) вызвать `wgctrl.ConfigureDevice`, 3) вернуть `listen_port` интерфейса. |
| 2 | Удалить пира  | `RemovePeer` должен удалять peer по `public_key`.                                                                 |
| 3 | Список пиров  | Сервис обязан уметь отдать текущий перечень для проверки консистентности системой.                                |
| 4 | Безопасность  | Принимать соединения **только** по TLS 1.3 с клиентским сертификатом, выданным внутренним CA.                     |
| 5 | Логирование   | stdout в формате JSON (`level`, `msg`, `ts`, `caller`).                                                           |
| 6 | Rate‑limit    | Не более 10 запросов в секунду; вернуть gRPC `RESOURCE_EXHAUSTED` при превышении.                                 |

### 1.5 Конфигурация (ENV)

| Переменная           | Пример                   | Описание                                       |
| -------------------- | ------------------------ | ---------------------------------------------- |
| `WG_AGENT_INTERFACE` | `wg0`                    | Интерфейс по умолчанию (можно override в RPC). |
| `WG_AGENT_TLS_CERT`  | `/etc/wg-agent/cert.pem` |                                                |
| `WG_AGENT_TLS_KEY`   | `/etc/wg-agent/key.pem`  |                                                |
| `WG_AGENT_CA_BUNDLE` | `/etc/wg-agent/ca.pem`   | Файл CA для client auth.                       |
| `WG_AGENT_ADDR`      | `0.0.0.0:7443`           | bind адрес.                                    |

### 1.6 Деплой (systemd)

```ini
[Unit]
Description=WireGuard gRPC agent
After=network-online.target

[Service]
ExecStart=/usr/local/bin/wg-agent
Restart=always
User=root      # нужен доступ к UAPI
AmbientCapabilities=CAP_NET_ADMIN

[Install]
WantedBy=multi-user.target
```

### 1.7 Тесты

* **unit**: мок `wgctrl.Client` (интерфейс), проверка валидации.
* **integration (local)**: запускаем `wg‑quick up test.conf`, гоняем gRPC‑запросы, сверяем `wg show`.

### 1.8 Acceptance‑критерии

1. Вручную добавленный peer появляется в `wg show` без рестарта интерфейса.
2. Удалённый peer исчезает.
3. При неверном public key сервис возвращает `INVALID_ARGUMENT`.
4. Несанкционированное соединение (нет client cert) — `TLS handshake error`.

---

## 2. Сервис №2: **bot‑service**

### 2.1 Назначение

* Принимает сообщения от пользователей и администраторов в Telegram.
* Обрабатывает платежи «вручную» (чек → approve/reject).
* Раздаёт конфиги и управляет peers через wg‑agent.

### 2.2 Технологии

* **Go 1.22**
* `telegram-bot-api` (long polling)
* gRPC‑клиент к wg‑agent (сгенерировать из `agent.proto`)
* БД — **sqlite** (в dev может быть SQLite).
* ORM — `gorm.io/gorm`, миграции — `golang-migrate`.
* Планировщик — `robfig/cron/v3`.

### 2.3 Схема БД

```
servers(id PK, name, address, ca_thumbprint, enabled bool)
interfaces(id PK, server_id FK, name, network cidr, last_ip inet)
plans(id PK, name, price_int, duration_days)
users(tg_id PK, username, phone, created_at)
payments(id PK, user_id FK, plan_id FK, amount, receipt_file_id, status, created_at)
subscriptions(id PK, user_id FK, plan_id FK, interface_id FK,
              allowed_ip inet, pubkey, privkey_encrypted, start_date, end_date,
              active bool, revoked_by, revoked_at)
admins(tg_id PK, role)
```

### 2.4 Основные сценарии

| # | Сценарий               | Действия сервиса                                                                                                                                                 |
| - | ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | `/start`               | Создать `users` (если нет), показать help.                                                                                                                       |
| 2 | `/plans`               | SELECT `plans`, ответ в чате + inline «Купить».                                                                                                                  |
| 3 | Покупка                | Создать `payments(pending)`, прислать реквизиты.                                                                                                                 |
| 4 | Пользователь шлёт фото | Save `file_id`, форвард в админ‑чат (inline ✅❌).                                                                                                                 |
| 5 | Admin ✅                | `payments→approved`; транзакция: выдать IP (interfaces.last\_ip+1) → создать subscriptions(active\:false) → gRPC `AddPeer` → active=true → отправить .conf + QR. |
| 6 | Admin ❌                | `payments→rejected`; уведомить юзера.                                                                                                                            |
| 7 | Отключение             | Админ жмёт «❌ Disable»; gRPC `RemovePeer`, `active=false`.                                                                                                       |
| 8 | Cron`03:00`            | Напоминания за 3 дня до `end_date`; ещё одна задача — авто‑RemovePeer для просроченных.                                                                          |

### 2.5 Конфигурация (ENV)

| Var                 | Пример            | Описание               |
| ------------------- | ----------------- | ---------------------- |
| `BOT_TOKEN`         | `123:ABC`         | Telegram API token     |
| `BOT_ADMIN_CHAT_ID` | `-100123`         | Куда форвардятся чеки  |
| `DB_DSN`            | `postgres://…`    |                        |
| `AGENT_CA_BUNDLE`   | `/etc/bot/ca.pem` |                        |
| `REDIS_URL` (opt.)  | `redis://…`       | для rate‑limit / locks |

### 2.6 gRPC‑клиент к агенту

* На каждый `servers.enabled` открываем пул соединений (по ca\_thumbprint).
* Повторяем запрос 3 раза с эксп. back‑off (50 ms · 2^n).
* Ошибки `RESOURCE_EXHAUSTED` — ждём и повторяем.

### 2.7 Логика выдачи IP

```sql
WITH next AS (
  SELECT id, last_ip + 1 AS new_ip
  FROM interfaces i
  LEFT JOIN subscriptions s USING(interface_id)
  WHERE enabled
  GROUP BY id
  ORDER BY COUNT(s.*) ASC, id
  LIMIT 1 FOR UPDATE
)
UPDATE interfaces i
SET last_ip = next.new_ip
FROM next
WHERE i.id = next.id
RETURNING i.id, next.new_ip;
```

### 2.8 Логирование и метрики

* stdout JSON (`{"level":"info","ts":...,"msg":"payment approved"}`)
* Prometheus endpoint `/metrics` (latency gRPC, count payments, active subs).
  Библиотека `promhttp`.

### 2.9 Тесты

* unit: бизнес‑логика Telegram handler’ов (табличные тесты).
* integration‑tests: поднять Postgres (test‑container), запустить mock‑agent (go‑tests), пройти сценарий Approve.

### 2.10 Acceptance‑критерии

1. Новый пользователь проходит сценарий покупки за ≤ 30 сек (manual QA).
2. При одновременном approve двух админов создаётся **одна** запись subscriptions (констр. UNIQUE(payment\_id)).
3. Отказ агент‑сервера → бот повторяет запрос на другой хост или сообщает администратору.
4. Просроченный subscription удаляется из WG‑серверов максимум через 10 минут после `end_date`.

---

## 3. Общие задачи DevOps

| № | Задача                    | Критерий готовности                              |
| - | ------------------------- | ------------------------------------------------ |
| 1 | CI GitHub Actions         | go test + go vet + staticcheck + build binaries. |
| 2 | Dockerfile обеих сервисов | образ < 30 MB, non‑root для bot‑service.         |
| 3 | Helm‑чарты (k8s)          | values: replicas, secrets, resources.            |
| 4 | CA in‑house               | скрипт `make-ca.sh` → выдаёт cert bot / agents.  |

---

## 4. Timeline (реалистично для Junior)

| Неделя | Что делаем                                         | Результат             |
| ------ | -------------------------------------------------- | --------------------- |
| 1      | wg‑agent: proto + AddPeer/RemovePeer + unit‑тесты  | локально управляет WG |
| 2      | TLS + rate‑limit + Docker + systemd                | агент готов к проду   |
| 3      | bot‑service: БД‑схема + миграции + `/start /plans` | бот отвечает          |
| 4      | Платёжный flow + gRPC к агенту                     | полный happy‑path     |
| 5      | Cron‑таски + disable + метрики                     | MVP feature‑complete  |
| 6      | Документация + Helm + нагрузочный тест             | релиз 1.0             |

---

## 5. Что сдать в конце

* Репозиторий **github.com/our‑org/wg‑project**

  * директории `/agent` и `/bot`
  * `README.md` с командами `make dev`, `make proto`, `make run‑agent`
* Образы `ghcr.io/our‑org/wg‑agent:1.0` , `…/wg‑bot:1.0`
* SQL миграции в `migrations/`
* Helm‑чарт `charts/wg‑project`
* Файл `docs/postman_collection.json` — для ручного теста gRPC эквивалент через **gRPC Client**.

---

**Конец ТЗ**
