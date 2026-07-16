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

echo "== polling IMAP (993) for delivery =="
found=0
for _ in $(seq 1 30); do
  # Real IMAPS login + SEARCH via imaplib (robust across the test-CA TLS).
  if IMAP_HOST="${imap_host}" IMAP_USER="${user}" IMAP_PASS="${password}" \
     IMAP_TOKEN="${token}" python3 - <<'PY'
import imaplib, ssl, os, sys
ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE
try:
    m = imaplib.IMAP4_SSL(os.environ["IMAP_HOST"], 993, ssl_context=ctx)
    m.login(os.environ["IMAP_USER"], os.environ["IMAP_PASS"])
    m.select("INBOX")
    typ, data = m.search(None, "SUBJECT", os.environ["IMAP_TOKEN"])
    hit = bool(data and data[0].split())
    m.logout()
    sys.exit(0 if hit else 1)
except Exception as e:
    print("imap:", e, file=sys.stderr)
    sys.exit(2)
PY
  then
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
