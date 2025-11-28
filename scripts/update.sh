#!/bin/bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

INSTALL_DIR="/opt/wg-agent"
SERVICE_NAME="wg-agent"

# Go path
export PATH=$PATH:/usr/local/go/bin:/root/go/bin

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

usage() {
    echo "Использование:"
    echo "  $0 update         - обновить до последней версии"
    echo "  $0 switch <tag>   - переключить на версию (v0.0.2, v0.0.3, ...)"
    echo "  $0 list           - показать доступные версии"
    echo "  $0 current        - текущая версия"
    exit 1
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "Нужен root"
        exit 1
    fi
}

build_and_restart() {
    cd "$INSTALL_DIR"

    log_info "Остановка сервиса..."
    systemctl stop "$SERVICE_NAME" || true

    log_info "Сборка..."
    go build -o bin/wg-agent ./cmd/wg-agent

    log_info "Запуск сервиса..."
    systemctl start "$SERVICE_NAME"

    sleep 2
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        log_info "Готово!"
        systemctl status "$SERVICE_NAME" --no-pager | head -5
    else
        log_error "Сервис не запустился!"
        journalctl -u "$SERVICE_NAME" -n 10 --no-pager
        exit 1
    fi
}

do_update() {
    check_root
    cd "$INSTALL_DIR"

    log_info "Получение обновлений..."
    git fetch --all --tags --quiet

    local branch=$(git remote show origin | grep 'HEAD branch' | awk '{print $NF}')
    git checkout "$branch" --quiet
    git pull origin "$branch" --quiet

    log_info "Версия: $(git rev-parse --short HEAD)"
    build_and_restart
}

do_switch() {
    local tag="$1"
    if [[ -z "$tag" ]]; then
        log_error "Укажи тег: $0 switch v0.0.3"
        exit 1
    fi

    check_root
    cd "$INSTALL_DIR"

    git fetch --all --tags --quiet

    if ! git rev-parse "$tag" >/dev/null 2>&1; then
        log_error "Тег $tag не найден"
        echo "Доступные:"
        git tag -l --sort=-v:refname | head -10
        exit 1
    fi

    log_info "Переключение на $tag..."
    git checkout "$tag" --quiet

    build_and_restart
}

do_list() {
    cd "$INSTALL_DIR"
    git fetch --tags --quiet
    echo "Доступные версии:"
    git tag -l --sort=-v:refname | head -20
    echo ""
    echo "Текущая: $(git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD)"
}

do_current() {
    cd "$INSTALL_DIR"
    echo "$(git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD)"
}

case "${1:-}" in
    update)  do_update ;;
    switch)  do_switch "$2" ;;
    list)    do_list ;;
    current) do_current ;;
    *)       usage ;;
esac