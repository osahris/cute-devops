#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
# SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
#
# SPDX-License-Identifier: EUPL-1.2

set -euo pipefail

# --- Configuration ---
ALERTA_API_URL="${ALERTA_API_URL:-http://localhost:8080/api}"
ALERTA_API_KEY="${ALERTA_API_KEY:-demo-api-key}"
BACKUP_RESOURCE="${BACKUP_RESOURCE:-$(hostname)}"
BACKUP_ENVIRONMENT="${BACKUP_ENVIRONMENT:-Development}"
HEARTBEAT_TIMEOUT="${HEARTBEAT_TIMEOUT:-600}"        # 10 min — max expected backup duration
HEARTBEAT_INTERVAL="${HEARTBEAT_INTERVAL:-90000}"    # 25 hours — until next scheduled run

# --- Helper functions ---

send_heartbeat() {
    local timeout="$1"
    local attributes
    attributes=$(jo -- -s environment="${BACKUP_ENVIRONMENT}" service="$(jo -a Backup)" -s group="Backup" -s severity="critical")
    jo -- -s origin="backup/${BACKUP_RESOURCE}" tags="$(jo -a backup)" timeout="${timeout}" \
       attributes="${attributes}" |
    curl -sf -o /dev/null -X POST "${ALERTA_API_URL}/heartbeat" \
        -H "Authorization: Key ${ALERTA_API_KEY}" \
        -H "Content-Type: application/json" \
        -d @-
}

send_alert() {
    local event="$1"
    local severity="$2"
    local text="$3"
    local correlate="${4:-}"
    local args=(
        resource="${BACKUP_RESOURCE}" event="${event}" environment="${BACKUP_ENVIRONMENT}"
        severity="${severity}" service="$(jo -a Backup)" text="${text}"
    )
    if [ -n "${correlate}" ]; then
        args+=(correlate="${correlate}")
    fi
    jo "${args[@]}" |
    curl -sf -o /dev/null -X POST "${ALERTA_API_URL}/alert" \
        -H "Authorization: Key ${ALERTA_API_KEY}" \
        -H "Content-Type: application/json" \
        -d @-
}

# --- Main ---

echo "Starting backup for resource: ${BACKUP_RESOURCE}"

# Send start heartbeat (short timeout — if backup hangs, this expires)
send_heartbeat "${HEARTBEAT_TIMEOUT}"
send_alert "BackupStarted" "informational" "Backup started on ${BACKUP_RESOURCE}" "$(jo -a BackupStarted BackupCompleted BackupFailed)"

# Run the backup command
echo "Running backup..."
BACKUP_COMMAND="${BACKUP_COMMAND:-echo 'no backup command configured'}"
backup_exit_code=0
(eval "${BACKUP_COMMAND}") || backup_exit_code=$?

if [ "${backup_exit_code}" -eq 0 ]; then
    echo "Backup completed successfully"
    send_alert "BackupCompleted" "normal" "Backup completed successfully on ${BACKUP_RESOURCE}" "$(jo -a BackupStarted BackupCompleted BackupFailed)"
    # Send completion heartbeat (long timeout — until next scheduled run)
    send_heartbeat "${HEARTBEAT_INTERVAL}"
else
    echo "Backup FAILED with exit code ${backup_exit_code}" >&2
    send_alert "BackupFailed" "critical" "Backup failed on ${BACKUP_RESOURCE} (exit code: ${backup_exit_code})" "$(jo -a BackupStarted BackupCompleted BackupFailed)"
    exit "${backup_exit_code}"
fi
