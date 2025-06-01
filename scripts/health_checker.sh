#!/bin/bash

URL="http://127.0.0.1:8080/health"
LOGFILE="/var/log/wg-agent_health.log"

STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$URL")
if [ "$STATUS" -ne 200 ]; then
  echo "$(date): wg-agent healthcheck failed with status $STATUS" >> "$LOGFILE"
  if [ -n "$TG_TOKEN" ] && [ -n "$TG_CHAT_ID" ]; then
    curl -s -X POST "https://api.telegram.org/bot$TG_TOKEN/sendMessage" \
      -d chat_id="$TG_CHAT_ID" \
      -d text="🔥 ALERT: wg-agent responded $STATUS at $(date)"
  fi
fi
