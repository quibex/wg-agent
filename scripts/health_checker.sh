#!/bin/bash

# Health checker for wg-agent service
# Sends/updates status messages to your personal Telegram account
# Required environment variables: TG_TOKEN, TG_CHAT_ID

SERVICE_NAME="wg-agent"
HEALTH_URL="http://localhost:8080/health"
MESSAGE_ID_FILE="/tmp/wg-agent-message-id"
CURRENT_TIME=$(date '+%Y-%m-%d %H:%M:%S')
HOSTNAME=$(hostname)

# Function to send new message
send_message() {
    local message="$1"
    if [ -n "$TG_TOKEN" ] && [ -n "$TG_CHAT_ID" ]; then
        response=$(curl -s -X POST "https://api.telegram.org/bot$TG_TOKEN/sendMessage" \
                        -d "chat_id=$TG_CHAT_ID" \
                        -d "text=$message" \
                        -d "parse_mode=HTML")
        
        # Extract message_id from response and save it
        message_id=$(echo "$response" | grep -o '"message_id":[0-9]*' | cut -d':' -f2)
        if [ -n "$message_id" ]; then
            echo "$message_id" > "$MESSAGE_ID_FILE"
        fi
    fi
}

# Function to edit existing message
edit_message() {
    local message="$1"
    local message_id="$2"
    if [ -n "$TG_TOKEN" ] && [ -n "$TG_CHAT_ID" ] && [ -n "$message_id" ]; then
        curl -s -X POST "https://api.telegram.org/bot$TG_TOKEN/editMessageText" \
             -d "chat_id=$TG_CHAT_ID" \
             -d "message_id=$message_id" \
             -d "text=$message" \
             -d "parse_mode=HTML" > /dev/null
    fi
}

# Check service health
if curl -s -f "$HEALTH_URL" > /dev/null; then
    # Service is OK
    OK_MESSAGE="✅ <b>$SERVICE_NAME</b> работает нормально

🖥 Сервер: <code>$HOSTNAME</code>
🕒 Обновлено: <code>$CURRENT_TIME</code>
📊 Статус: <b>OK</b>"

    # Try to edit existing message, or send new one if no message_id
    if [ -f "$MESSAGE_ID_FILE" ]; then
        message_id=$(cat "$MESSAGE_ID_FILE")
        edit_message "$OK_MESSAGE" "$message_id"
        
        # If edit failed (message too old or deleted), send new message
        if [ $? -ne 0 ]; then
            send_message "$OK_MESSAGE"
        fi
    else
        # No previous message, send new one
        send_message "$OK_MESSAGE"
    fi
    
    exit 0
else
    # Service is DOWN - always send new message
    ERROR_MESSAGE="🚨 <b>$SERVICE_NAME НЕДОСТУПЕН!</b>

🖥 Сервер: <code>$HOSTNAME</code>
🕒 Время ошибки: <code>$CURRENT_TIME</code>
❌ Статус: <b>FAILED</b>
🔗 URL: <code>$HEALTH_URL</code>"

    send_message "$ERROR_MESSAGE"
    
    # Remove message_id file so next OK will be a new message
    rm -f "$MESSAGE_ID_FILE"
    
    exit 1
fi
