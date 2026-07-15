#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
# SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
#
# SPDX-License-Identifier: EUPL-1.2

set -uo pipefail

smtp_host="${1:?smtp host}"
imap_host="${2:?imap host}"
user="${3:?user}"
password="${4:?password}"

passed=0
failed=0
pass() { echo "PASS: $*"; passed=$((passed + 1)); }
fail() { echo "FAIL: $*"; failed=$((failed + 1)); }

token="cutedevops-test-$$-${RANDOM}"

echo "== injecting test message (subject token: ${token}) =="
if swaks --server "${smtp_host}" --port 25 \
        --to "${user}" --from "test@$(hostname -f 2>/dev/null || echo localhost)" \
        --header "Subject: ${token}" --body "harness probe ${token}" \
        --timeout 20 >/dev/null 2>&1; then
  pass "SMTP submission accepted"
else
  fail "SMTP submission rejected"
fi

echo "== polling IMAP for delivery =="
found=0
for _ in $(seq 1 30); do
  # Fetch recent message subjects over IMAPS and look for our token.
  if curl -s --insecure --max-time 10 \
        --url "imaps://${imap_host}/INBOX" \
        --user "${user}:${password}" \
        --request "SEARCH SUBJECT ${token}" 2>/dev/null | grep -q '[0-9]'; then
    found=1
    break
  fi
  sleep 2
done
if [ "${found}" -eq 1 ]; then
  pass "message delivered and retrievable over IMAP"
else
  fail "message not found in mailbox within timeout"
fi

echo "== Results: ${passed} passed, ${failed} failed =="
[ "${failed}" -eq 0 ]
