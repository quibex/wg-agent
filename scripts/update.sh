#!/bin/bash
set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

INSTALL_DIR="/opt/wg-agent"
SERVICE_NAME="wg-agent"

usage() {
    echo "Управление версиями wg-agent"
    echo ""
    echo "Использование: $0 <команда> [версия]"
    echo ""
    echo "Команды:"
    echo "  update [tag]     Обновить до указанной версии (по умолчанию: latest)"
    echo "  rollback <tag>   Откатить до указанной версии"
    echo "  list             Показать доступные версии (теги)"
    echo "  current          Показать текущую версию"
    echo "  status           Показать статус сервиса"
    echo ""
    echo "Примеры:"
    echo "  $0 update              # Обновить до последней версии"
    echo "  $0 update v0.0.3       # Обновить до v0.0.3"
    echo "  $0 rollback v0.0.2     # Откатить до v0.0.2"
    echo "  $0 list                # Показать все теги"
    exit 1
}

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "Этот скрипт должен запускаться от root"
        exit 1
    fi
}

check_install_dir() {
    if [[ ! -d "$INSTALL_DIR" ]]; then
        log_error "Директория $INSTALL_DIR не найдена"
        exit 1
    fi
}

get_current_version() {
    cd "$INSTALL_DIR"
    git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD
}

list_versions() {
    cd "$INSTALL_DIR"
    log_info "Получение списка версий..."
    git fetch --tags --quiet

    echo ""
    echo "Доступные версии:"
    echo "─────────────────"
    git tag -l --sort=-v:refname | head -20
    echo ""
    echo "Текущая версия: $(get_current_version)"
}

show_current() {
    cd "$INSTALL_DIR"
    echo "Текущая версия: $(get_current_version)"
    echo "Коммит: $(git rev-parse HEAD)"
    echo "Дата: $(git log -1 --format=%ci)"
}

show_status() {
    systemctl status "$SERVICE_NAME" --no-pager
}

backup_current() {
    local backup_tag="backup-$(date +%Y%m%d-%H%M%S)"
    cd "$INSTALL_DIR"

    # Сохраняем текущий коммит для возможного отката
    local current_commit=$(git rev-parse HEAD)
    echo "$current_commit" > /tmp/wg-agent-last-commit

    log_info "Текущий коммит сохранён: $current_commit"
}

build_and_restart() {
    cd "$INSTALL_DIR"

    log_info "Генерация protobuf..."
    make proto

    log_info "Сборка..."
    make build

    log_info "Перезапуск сервиса..."
    systemctl restart "$SERVICE_NAME"

    sleep 2

    if systemctl is-active --quiet "$SERVICE_NAME"; then
        log_info "Сервис успешно запущен"
        systemctl status "$SERVICE_NAME" --no-pager | head -10
    else
        log_error "Сервис не запустился!"
        journalctl -u "$SERVICE_NAME" -n 20 --no-pager
        return 1
    fi
}

do_update() {
    local target_version="${1:-}"

    check_root
    check_install_dir
    cd "$INSTALL_DIR"

    log_info "Получение обновлений..."
    git fetch --all --tags --quiet

    backup_current

    if [[ -z "$target_version" ]]; then
        # Обновление до последнего коммита в main/master
        log_info "Обновление до последней версии..."

        local default_branch=$(git remote show origin | grep 'HEAD branch' | awk '{print $NF}')
        git checkout "$default_branch" --quiet
        git pull origin "$default_branch" --quiet

        target_version=$(get_current_version)
    else
        # Обновление до конкретного тега
        if ! git rev-parse "$target_version" >/dev/null 2>&1; then
            log_error "Версия $target_version не найдена"
            echo "Доступные версии:"
            git tag -l --sort=-v:refname | head -10
            exit 1
        fi

        log_info "Переключение на версию $target_version..."
        git checkout "$target_version" --quiet
    fi

    log_info "Версия: $target_version"

    build_and_restart

    log_info "Обновление завершено!"
}

do_rollback() {
    local target_version="$1"

    if [[ -z "$target_version" ]]; then
        # Попробовать откатить к предыдущему коммиту
        if [[ -f /tmp/wg-agent-last-commit ]]; then
            target_version=$(cat /tmp/wg-agent-last-commit)
            log_info "Откат к предыдущему коммиту: $target_version"
        else
            log_error "Укажите версию для отката"
            usage
        fi
    fi

    check_root
    check_install_dir
    cd "$INSTALL_DIR"

    log_info "Получение тегов..."
    git fetch --all --tags --quiet

    if ! git rev-parse "$target_version" >/dev/null 2>&1; then
        log_error "Версия $target_version не найдена"
        echo "Доступные версии:"
        git tag -l --sort=-v:refname | head -10
        exit 1
    fi

    backup_current

    log_info "Откат к версии $target_version..."
    git checkout "$target_version" --quiet

    build_and_restart

    log_info "Откат завершён!"
}

# Основная логика
case "${1:-}" in
    update)
        do_update "${2:-}"
        ;;
    rollback)
        do_rollback "${2:-}"
        ;;
    list)
        check_install_dir
        list_versions
        ;;
    current)
        check_install_dir
        show_current
        ;;
    status)
        show_status
        ;;
    *)
        usage
        ;;
esac