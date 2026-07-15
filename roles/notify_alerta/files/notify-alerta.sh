#!/bin/bash

# SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
# SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
#
# SPDX-License-Identifier: EUPL-1.2

set -e

# Load configuration from .env file
if [ -f /etc/checker/notify_alerta.env ]; then
    source /etc/checker/notify_alerta.env
else
    echo "Error: /etc/checker/notify_alerta.env not found" >&2
    exit 1
fi

# Parse arguments
hostname="$1"
check_id="$2"
exit_code="$3"

# Map exit code to severity
case $exit_code in
    0) severity="ok" ;;
    1) severity="warning" ;;
    2) severity="critical" ;;
    3) severity="unknown" ;;
    *) severity="debug" ;;
esac

# Build curl command with optional auth header
CURL_OPTS="-s -X POST -H \"Content-Type: application/json\""
if [ -n "$ALERTA_API_KEY" ]; then
    CURL_OPTS="$CURL_OPTS -H \"Authorization: Key $ALERTA_API_KEY\""
fi

cat \
| jo \
    text=@- \
    resource="$hostname" \
    event="$check_id" \
    environment="$ALERTA_ENVIRONMENT" \
    severity="$severity" \
    value="$exit_code" \
    service="[\"$hostname\"]" \
    origin=checker \
    type=checkerCheck \
| eval curl $CURL_OPTS \
    -d @- \
    --fail-with-body \
    "$ALERTA_API_ALERT_URL"
