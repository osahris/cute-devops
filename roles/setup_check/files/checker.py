#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2024-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-License-Identifier: EUPL-1.2

import sys
import subprocess


def main():
    # Ensure required variables are set
    if len(sys.argv) < 2:
        print("Error: Usage: checker.py <run_file>", file=sys.stderr)
        sys.exit(9)
    
    run_file = sys.argv[1]
    
    # Run check.sh and capture output
    with open(run_file, 'w') as output_file:
        process = subprocess.Popen(
            ['./check.sh'],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True
        )
        
        # Tee output to both file and stdout
        for line in process.stdout:
            sys.stdout.write(line)
            sys.stdout.flush()
            output_file.write(line)
            output_file.flush()
        
        # Wait for process to complete and get exit code
        exit_code = process.wait()
    
    sys.exit(exit_code)


if __name__ == "__main__":
    main()