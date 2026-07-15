#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
# SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
#
# SPDX-License-Identifier: EUPL-1.2

set -uo pipefail

ALERTA_API_URL="${ALERTA_API_URL:-http://localhost:8080/api}"
ALERTA_API_KEY="${ALERTA_API_KEY:-demo-api-key}"
BACKUP_RESOURCE="${BACKUP_RESOURCE:-testserver}"

passed=0
failed=0

run_backup() {
    local desc="$1"
    local cmd="$2"
    local expect_fail="${3:-false}"

    echo "=== ${desc} ==="
    local exit_code=0
    BACKUP_COMMAND="${cmd}" /backup.sh || exit_code=$?

    if [ "${expect_fail}" = "true" ] && [ "${exit_code}" -ne 0 ]; then
        echo "PASS (expected failure)"
        ((passed++))
    elif [ "${expect_fail}" = "false" ] && [ "${exit_code}" -eq 0 ]; then
        echo "PASS (expected success)"
        ((passed++))
    else
        echo "FAIL (expect_fail=${expect_fail}, exit_code=${exit_code})"
        ((failed++))
    fi
    echo
}

show_alerts() {
    local response
    response=$(curl -s "${ALERTA_API_URL}/alerts?resource=${BACKUP_RESOURCE}" \
        -H "Authorization: Key ${ALERTA_API_KEY}")
    echo "${response}" | python3 -c "
import sys, json
alerts = json.load(sys.stdin)['alerts']
if not alerts:
    print('  (no alerts)')
for a in alerts:
    print(f'  {a[\"event\"]:20s} {a[\"severity\"]:14s} {a[\"status\"]:10s} dup={a[\"duplicateCount\"]}')
"
    echo
}

# Wait for alerta to be ready
echo "Waiting for Alerta..."
for i in $(seq 1 30); do
    curl -sf "${ALERTA_API_URL}/" > /dev/null 2>&1 && break
    sleep 1
done


show_heartbeats() {
    local response
    response=$(curl -s "${ALERTA_API_URL}/heartbeats" \
        -H "Authorization: Key ${ALERTA_API_KEY}")
    echo "${response}" | python3 -c "
import sys, json
heartbeats = json.load(sys.stdin)['heartbeats']
if not heartbeats:
    print('  (no heartbeats)')
for h in heartbeats:
    print(f'  {h[\"origin\"]:25s} status={h[\"status\"]:10s} timeout={h[\"timeout\"]}')
"
    echo
}

expect_alert() {
    local resource="$1"
    local expected_event="$2"
    local expected_severity="$3"
    local expected_status="$4"
    local response
    response=$(curl -s "${ALERTA_API_URL}/alerts?resource=${resource}" \
        -H "Authorization: Key ${ALERTA_API_KEY}")
    local actual
    actual=$(echo "${response}" | python3 -c "
import sys, json
alerts = json.load(sys.stdin)['alerts']
if not alerts:
    print('none none none')
else:
    a = alerts[0]
    print(f'{a[\"event\"]} {a[\"severity\"]} {a[\"status\"]}')
")
    local expected="${expected_event} ${expected_severity} ${expected_status}"
    if [ "${actual}" = "${expected}" ]; then
        echo "  PASS alert: ${actual}"
        ((passed++))
    else
        echo "  FAIL alert: expected '${expected}', got '${actual}'"
        ((failed++))
    fi
}

clear_alerts() {
    local ids
    ids=$(curl -s "${ALERTA_API_URL}/alerts?resource=${BACKUP_RESOURCE}" \
        -H "Authorization: Key ${ALERTA_API_KEY}" | \
        python3 -c "
import sys, json
for a in json.load(sys.stdin)['alerts']:
    print(a['id'])" 2>/dev/null) || true
    for id in ${ids}; do
        curl -sf -o /dev/null -X DELETE "${ALERTA_API_URL}/alert/${id}" \
            -H "Authorization: Key ${ALERTA_API_KEY}"
    done
}

clear_heartbeats() {
    local ids
    ids=$(curl -s "${ALERTA_API_URL}/heartbeats" \
        -H "Authorization: Key ${ALERTA_API_KEY}" | \
        python3 -c "
import sys, json
for h in json.load(sys.stdin)['heartbeats']:
    print(h['id'])" 2>/dev/null) || true
    for id in ${ids}; do
        curl -sf -o /dev/null -X DELETE "${ALERTA_API_URL}/heartbeat/${id}" \
            -H "Authorization: Key ${ALERTA_API_KEY}"
    done
}

# Test scenarios
echo "--- Test 1: Successful backup ---"
clear_alerts; clear_heartbeats
run_backup "Successful backup" "sleep 1" false
expect_alert "${BACKUP_RESOURCE}" "BackupCompleted" "normal" "closed"
show_alerts

sleep 10

echo "--- Test 2: Failed backup ---"
clear_alerts; clear_heartbeats
run_backup "Failed backup" "exit 1" true
expect_alert "${BACKUP_RESOURCE}" "BackupFailed" "critical" "open"
show_alerts

sleep 10

echo "--- Test 3: Recovery after failure ---"
run_backup "Recovery" "sleep 1" false
expect_alert "${BACKUP_RESOURCE}" "BackupCompleted" "normal" "closed"
show_alerts

sleep 10

echo "--- Test 4: Silent kill (backup killed mid-run) ---"
clear_alerts; clear_heartbeats
echo "  Starting backup that will be killed..."
BACKUP_COMMAND="sleep 300" HEARTBEAT_TIMEOUT=5 /backup.sh &
backup_pid=$!
sleep 2
echo "  Killing backup process (pid ${backup_pid})..."
kill -9 "${backup_pid}" 2>/dev/null || true
wait "${backup_pid}" 2>/dev/null || true
echo "  Backup killed. Heartbeat timeout is 5s."
echo "  Waiting for heartbeat to expire..."
show_heartbeats
sleep 8
echo "  After expiry:"
show_heartbeats
echo "  Waiting for heartbeat monitor to detect it..."
sleep 65
expect_alert "backup/${BACKUP_RESOURCE}" "HeartbeatFail" "major" "open"
show_alerts

sleep 10

echo "--- Test 5: Recovery after silent kill ---"
run_backup "Recovery after kill" "sleep 1" false
expect_alert "${BACKUP_RESOURCE}" "BackupCompleted" "normal" "closed"
show_alerts

sleep 10

echo "=== Results: ${passed} passed, ${failed} failed ==="
[ "${failed}" -eq 0 ]
