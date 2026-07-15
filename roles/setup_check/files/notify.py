#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2024-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-License-Identifier: EUPL-1.2

import os
import sys
import glob
import subprocess
from concurrent.futures import ProcessPoolExecutor, as_completed


def run_notifier(script, hostname, check_id, exit_code, run_file_content):
    """Run a single notifier script and return its exit code."""
    try:
        print(f"Notifying with {script}")
        process = subprocess.Popen(
            [script, hostname, check_id, str(exit_code)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
        )
        stdout, stderr = process.communicate(input=run_file_content)
        
        if stdout:
            print(stdout, end='')
        if stderr:
            print(stderr, end='', file=sys.stderr)
            
        if process.returncode != 0:
            print(f"A notifier ({script}) failed with exit code {process.returncode}")
            return 8
        return 0
    except Exception as e:
        print(f"Error running notifier {script}: {e}", file=sys.stderr)
        return 8


def main():
    # Get input parameters
    if len(sys.argv) < 4:
        print("Usage: notify.py <hostname> <check_id> <run_file>", file=sys.stderr)
        sys.exit(1)
        
    hostname = sys.argv[1]
    check_id = sys.argv[2]
    run_file = sys.argv[3]
    exit_code = os.environ.get('EXIT_STATUS', '0')
    
    # Read the run file content
    try:
        with open(run_file, 'r') as f:
            run_file_content = f.read()
    except IOError as e:
        print(f"Error reading run file: {e}", file=sys.stderr)
        sys.exit(1)
    
    # Find all executable notifier scripts
    notifier_scripts = []
    for script in glob.glob('/etc/checker/notifiers/*.sh'):
        if os.path.isfile(script) and os.access(script, os.X_OK):
            notifier_scripts.append(script)
    
    if not notifier_scripts:
        print("No executable notifier scripts found")
        sys.exit(0)
    
    # Run notifiers in parallel
    failed = 0
    with ProcessPoolExecutor() as executor:
        futures = {
            executor.submit(run_notifier, script, hostname, check_id, exit_code, run_file_content): script
            for script in notifier_scripts
        }
        
        for future in as_completed(futures):
            result = future.result()
            if result != 0:
                failed = result
    
    sys.exit(failed)


if __name__ == "__main__":
    main()