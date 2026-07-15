#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: EUPL-1.2
#
# Throwaway test PKI for the test-in-containers harness. Generates a
# local CA and one leaf cert per instance so the mail roles can present
# real, mutually-trusted certs (STARTTLS, IMAPS, cross-instance
# LMTP/SASL) without ACME. This is test infrastructure ONLY — it is not
# the certificate-role rework and must never be used for production.
#
# Usage: make-ca.sh <domain> <instance> [<instance> ...]
#   e.g. make-ca.sh mail.test mx mo mb ml
#
# Output goes to test/ca/out/ (gitignored). Re-running is idempotent:
# existing CA and leaf certs are kept.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
out="${here}/out"
mkdir -p "${out}"

domain="${1:?domain required}"
shift
[ "$#" -ge 1 ] || { echo "at least one instance name required" >&2; exit 2; }

ca_key="${out}/ca.key.pem"
ca_crt="${out}/ca.crt.pem"

if [ ! -f "${ca_crt}" ]; then
  echo "· generating test CA"
  openssl genrsa -out "${ca_key}" 4096
  openssl req -x509 -new -nodes -key "${ca_key}" -sha256 -days 3650 \
    -subj "/O=cute-devops test/CN=cute-devops test CA" \
    -out "${ca_crt}"
fi

for inst in "$@"; do
  key="${out}/${inst}.key.pem"
  crt="${out}/${inst}.crt.pem"
  [ -f "${crt}" ] && { echo "· ${inst}: cert exists, keeping"; continue; }
  echo "· issuing cert for ${inst}"
  openssl genrsa -out "${key}" 2048
  san="DNS:${inst},DNS:${domain},DNS:${inst}.${domain}"
  csr="${out}/${inst}.csr.pem"
  openssl req -new -key "${key}" -subj "/O=cute-devops test/CN=${inst}.${domain}" -out "${csr}"
  openssl x509 -req -in "${csr}" -CA "${ca_crt}" -CAkey "${ca_key}" \
    -CAcreateserial -days 825 -sha256 \
    -extfile <(printf 'subjectAltName=%s\nbasicConstraints=CA:FALSE\nkeyUsage=digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\n' "${san}") \
    -out "${crt}"
  # fullchain = leaf + CA, what postfix/dovecot want as their cert file
  cat "${crt}" "${ca_crt}" > "${out}/${inst}.fullchain.pem"
  rm -f "${csr}"
done

echo "test CA and leaf certs are in ${out}"
