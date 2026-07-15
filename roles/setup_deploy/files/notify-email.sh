#!/bin/bash
# SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: EUPL-1.2
# Email notifier for deploy notifications

set -euo pipefail

# Arguments
HOSTNAME="${1:-}"
DEPLOY_ID="${2:-}"
EXIT_CODE="${3:-}"

# Source configuration
CONFIG_FILE="/etc/deploy/notify_email.env"
if [[ -f "$CONFIG_FILE" ]]; then
    source "$CONFIG_FILE"
fi

# Configuration with defaults
NOTIFY_EMAIL_TO="${NOTIFY_EMAIL_TO:-}"
NOTIFY_EMAIL_FROM="${NOTIFY_EMAIL_FROM:-deploy@$HOSTNAME}"
NOTIFY_EMAIL_ON_SUCCESS="${NOTIFY_EMAIL_ON_SUCCESS:-false}"
NOTIFY_EMAIL_ON_FAILURE="${NOTIFY_EMAIL_ON_FAILURE:-true}"
NOTIFY_EMAIL_MAX_OUTPUT_LINES="${NOTIFY_EMAIL_MAX_OUTPUT_LINES:-100}"
NOTIFY_EMAIL_RATE_LIMIT="${NOTIFY_EMAIL_RATE_LIMIT:-10}"

# Validate required configuration
if [[ -z "$NOTIFY_EMAIL_TO" ]]; then
    echo "Email notification disabled: NOTIFY_EMAIL_TO not set"
    exit 0
fi

# Check if we should send notification based on exit code
should_notify=false
if [[ "$EXIT_CODE" -eq 0 && "$NOTIFY_EMAIL_ON_SUCCESS" == "true" ]]; then
    should_notify=true
    status="SUCCESS"
    subject="[Deploy] SUCCESS: $DEPLOY_ID on $HOSTNAME"
elif [[ "$EXIT_CODE" -ne 0 && "$NOTIFY_EMAIL_ON_FAILURE" == "true" ]]; then
    should_notify=true
    status="FAILURE"
    subject="[Deploy] FAILURE: $DEPLOY_ID on $HOSTNAME (exit code: $EXIT_CODE)"
else
    echo "Email notification skipped: exit code $EXIT_CODE not configured for notification"
    exit 0
fi

if [[ "$should_notify" != "true" ]]; then
    exit 0
fi

# Rate limiting
RATE_LIMIT_DIR="/var/lib/deploy/notify_email"
RATE_LIMIT_FILE="$RATE_LIMIT_DIR/${DEPLOY_ID}.rate"
mkdir -p "$RATE_LIMIT_DIR"

# Clean up old rate limit files (older than 1 hour)
find "$RATE_LIMIT_DIR" -name "*.rate" -mmin +60 -delete 2>/dev/null || true

# Count recent notifications
recent_count=$(find "$RATE_LIMIT_DIR" -name "${DEPLOY_ID}.rate" -mmin -60 2>/dev/null | wc -l)

if [[ $recent_count -ge $NOTIFY_EMAIL_RATE_LIMIT ]]; then
    echo "Email notification skipped: rate limit exceeded ($recent_count >= $NOTIFY_EMAIL_RATE_LIMIT per hour)"
    exit 0
fi

# Create rate limit file
touch "$RATE_LIMIT_FILE"

# Capture output (limited to max lines)
output=""
if [[ -p /dev/stdin ]]; then
    output=$(head -n "$NOTIFY_EMAIL_MAX_OUTPUT_LINES")
    if [[ $(echo "$output" | wc -l) -eq $NOTIFY_EMAIL_MAX_OUTPUT_LINES ]]; then
        output="$output
... (output truncated to $NOTIFY_EMAIL_MAX_OUTPUT_LINES lines)"
    fi
fi

# Compose email body
body="Deploy Status: $status
Hostname: $HOSTNAME
Deploy ID: $DEPLOY_ID
Exit Code: $EXIT_CODE
Timestamp: $(date -u '+%Y-%m-%d %H:%M:%S UTC')

"

if [[ -n "$output" ]]; then
    body="$body
=== Deploy Output ===
$output"
fi

# Send email
if command -v mail >/dev/null 2>&1; then
    echo "$body" | mail -s "$subject" -r "$NOTIFY_EMAIL_FROM" "$NOTIFY_EMAIL_TO"
    echo "Email notification sent to $NOTIFY_EMAIL_TO"
elif command -v sendmail >/dev/null 2>&1; then
    {
        echo "To: $NOTIFY_EMAIL_TO"
        echo "From: $NOTIFY_EMAIL_FROM"
        echo "Subject: $subject"
        echo ""
        echo "$body"
    } | sendmail "$NOTIFY_EMAIL_TO"
    echo "Email notification sent to $NOTIFY_EMAIL_TO"
else
    echo "Email notification failed: neither 'mail' nor 'sendmail' command found" >&2
    exit 1
fi