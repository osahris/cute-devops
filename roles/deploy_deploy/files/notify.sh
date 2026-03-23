#!/bin/bash
# SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Deploy notification dispatcher
# Runs all executable scripts in /etc/deploy/notifiers/ in parallel

set -euo pipefail

# Arguments
HOSTNAME="${1:-}"
DEPLOY_ID="${2:-}"
EXIT_CODE="${3:-}"
OUTPUT_FILE="${4:-}"

# Validate arguments
if [[ -z "$HOSTNAME" || -z "$DEPLOY_ID" || -z "$EXIT_CODE" ]]; then
    echo "Usage: $0 <hostname> <deploy_id> <exit_code> [output_file]" >&2
    exit 1
fi

# Directory containing notifier scripts
NOTIFIERS_DIR="/etc/deploy/notifiers"

# Create notifiers directory if it doesn't exist
mkdir -p "$NOTIFIERS_DIR"

# Find all executable files in the notifiers directory
notifiers=($(find "$NOTIFIERS_DIR" -type f -executable 2>/dev/null | sort))

echo "Deploy '$DEPLOY_ID' finished with exit code $EXIT_CODE"

if [[ ${#notifiers[@]} -eq 0 ]]; then
    echo "No notifiers found in $NOTIFIERS_DIR"
    exit 0
fi

echo "Running ${#notifiers[@]} notifier(s) for deploy '$DEPLOY_ID' (exit code: $EXIT_CODE)"

# Array to store background job PIDs
pids=()

# Run each notifier in the background
for notifier in "${notifiers[@]}"; do
    echo "Starting notifier: $(basename "$notifier")"
    
    # If output file exists, pipe it to the notifier; otherwise just run the notifier
    if [[ -n "$OUTPUT_FILE" && -f "$OUTPUT_FILE" ]]; then
        cat "$OUTPUT_FILE" | "$notifier" "$HOSTNAME" "$DEPLOY_ID" "$EXIT_CODE" &
    else
        "$notifier" "$HOSTNAME" "$DEPLOY_ID" "$EXIT_CODE" < /dev/null &
    fi
    
    pids+=($!)
done

# Wait for all notifiers to complete and track failures
failed=0
for i in "${!pids[@]}"; do
    pid=${pids[$i]}
    notifier=${notifiers[$i]}
    
    if wait "$pid"; then
        echo "Notifier completed successfully: $(basename "$notifier")"
    else
        echo "Notifier failed: $(basename "$notifier")" >&2
        failed=1
    fi
done

# Exit with code 8 if any notifier failed
if [[ $failed -eq 1 ]]; then
    echo "One or more notifiers failed" >&2
    exit 8
fi

echo "All notifiers completed successfully"
exit 0