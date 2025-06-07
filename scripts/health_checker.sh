#!/bin/bash

# Health checker for wg-agent service
# Sends notifications to your personal Telegram account if service is down
# Required environment variables: TG_TOKEN, TG_CHAT_ID

SERVICE_NAME="wg-agent"
HEALTH_URL="http://localhost:8080/health"

if ! curl -s -f "$HEALTH_URL" > /dev/null; then
    MESSAGE="🚨 $SERVICE_NAME health check failed on $(hostname)"
    
    if [ -n "$TG_TOKEN" ] && [ -n "$TG_CHAT_ID" ]; then
        curl -s -X POST "https://api.telegram.org/bot$TG_TOKEN/sendMessage" \
             -d "chat_id=$TG_CHAT_ID" \
             -d "text=$MESSAGE" > /dev/null
    fi
    
    exit 1
fi

exit 0
