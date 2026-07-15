#!/bin/sh

# SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: EUPL-1.2

# Get input parameters
hostname="$1"
check_id="$2"
run_file="$3"
exit_code="${EXIT_STATUS}"

failed=0
pids=""

# Run each notifier script in parallel
for script in /etc/checker/notifiers/*.sh; do
    if [ -x "$script" ]; then
        echo "Notifying with $script"
        cat "$run_file" | "$script" "$hostname" "$check_id" "$exit_code" & 
        pids="$pids $!"
    fi
done

# Wait for all notifiers and check their exit codes
for pid in $pids; do
    wait "$pid"
    notifier_exit=$?
    if [ $notifier_exit -ne 0 ]; then
        echo "A notifier (PID: $pid) failed with exit code $notifier_exit"
        failed=8
    fi
done

exit $failed
