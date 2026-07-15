#!/bin/bash

# SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
# SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
#
# SPDX-License-Identifier: EUPL-1.2

# Ensure required variables are set
if [ -z "$1" ]; then
    echo "Error: Usage: $0 <run_file>" >&2
    exit 9
fi

RUN_FILE="$1"

# Run check and tee output, using PIPESTATUS to get check.sh's exit code
./check.sh 2>&1 | tee "${RUN_FILE}"
exit ${PIPESTATUS[0]}
