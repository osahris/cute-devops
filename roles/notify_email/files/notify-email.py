#!/usr/bin/env python3

# SPDX-FileCopyrightText: 2024 - 2025 Uniklinik Köln
# SPDX-FileCopyrightText: 2024 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
#
# SPDX-License-Identifier: EUPL-1.2

import os
import sys
import time
import subprocess
from datetime import datetime
from pathlib import Path


def load_config():
    """Load configuration from /etc/checker/notify_email.env."""
    config = {}
    config_file = "/etc/checker/notify_email.env"
    
    if not os.path.exists(config_file):
        print(f"Error: {config_file} not found", file=sys.stderr)
        sys.exit(1)
    
    with open(config_file, 'r') as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith('#') and '=' in line:
                key, value = line.split('=', 1)
                # Remove quotes if present
                value = value.strip().strip('"').strip("'")
                config[key] = value
    
    return config


def get_status_name(status_code):
    """Map status codes to names."""
    status_map = {
        "0": "OK",
        "1": "WARNING",
        "2": "CRITICAL",
        "3": "UNKNOWN"
    }
    return status_map.get(status_code, "UNKNOWN")


def check_rate_limit(check_name, rate_limit, rate_window):
    """Check if we're within rate limits."""
    rate_dir = Path("/var/run/checker")
    rate_dir.mkdir(parents=True, exist_ok=True)
    rate_file = rate_dir / f"email_rate_{check_name}.txt"
    
    current_time = int(time.time())
    timestamps = []
    
    # Read existing timestamps
    if rate_file.exists():
        with open(rate_file, 'r') as f:
            for line in f:
                try:
                    timestamp = int(line.strip())
                    # Keep only timestamps within the window
                    if current_time - timestamp < int(rate_window):
                        timestamps.append(timestamp)
                except ValueError:
                    continue
    
    # Check if we've exceeded the limit
    if len(timestamps) >= int(rate_limit):
        print(f"Rate limit exceeded for {check_name} ({len(timestamps)}/{rate_limit} emails in {rate_window}s)")
        return False
    
    # Add current timestamp and save
    timestamps.append(current_time)
    with open(rate_file, 'w') as f:
        for ts in timestamps:
            f.write(f"{ts}\n")
    
    return True


def truncate_output(output, include_output, max_lines):
    """Truncate output if needed."""
    if include_output == "true":
        lines = output.split('\n')
        line_count = len(lines)
        max_lines = int(max_lines)
        
        if line_count > max_lines:
            truncated = '\n'.join(lines[:max_lines])
            return f"{truncated}\n\n... (truncated, showing first {max_lines} of {line_count} lines)"
        return output
    else:
        return "(Output suppressed)"


def send_email(subject, body, from_addr, to_addr):
    """Send email using mail command."""
    try:
        process = subprocess.Popen(
            ['mail', '-s', subject, '-r', from_addr, to_addr],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
        )
        stdout, stderr = process.communicate(input=body)
        
        if process.returncode != 0:
            print(f"Failed to send email: {stderr}", file=sys.stderr)
            return False
        return True
    except Exception as e:
        print(f"Error sending email: {e}", file=sys.stderr)
        return False


def main():
    """Main function."""
    # Load configuration
    config = load_config()
    
    # Get arguments
    hostname = sys.argv[1] if len(sys.argv) > 1 else "unknown"
    check_name = sys.argv[2] if len(sys.argv) > 2 else "unknown"
    status = sys.argv[3] if len(sys.argv) > 3 else "3"
    
    # Read output from stdin
    output = sys.stdin.read()
    timestamp = datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S UTC")
    
    # Get status name
    status_name = get_status_name(status)
    
    # Check if we should send notification for this status
    if status == "1" and config.get("NOTIFY_EMAIL_ON_WARNING", "true") != "true":
        sys.exit(0)
    elif status == "2" and config.get("NOTIFY_EMAIL_ON_CRITICAL", "true") != "true":
        sys.exit(0)
    
    # Rate limiting check
    rate_limit = config.get("NOTIFY_EMAIL_RATE_LIMIT", "5")
    rate_window = config.get("NOTIFY_EMAIL_RATE_WINDOW", "3600")
    if not check_rate_limit(check_name, rate_limit, rate_window):
        sys.exit(0)
    
    # Prepare output
    include_output = config.get("NOTIFY_EMAIL_INCLUDE_OUTPUT", "true")
    max_output_lines = config.get("NOTIFY_EMAIL_MAX_OUTPUT_LINES", "100")
    output = truncate_output(output, include_output, max_output_lines)
    
    # Generate email
    subject_prefix = config.get("NOTIFY_EMAIL_SUBJECT_PREFIX", "[Checker]")
    subject = f"{subject_prefix} {status_name}: {check_name} on {hostname}"
    
    body = f"""Monitoring Alert
================

Check:     {check_name}
Status:    {status_name} ({status})
Host:      {hostname}
Time:      {timestamp}

Output:
-------
{output}

---
This notification was generated by the checker monitoring system.
To modify notification settings, update /etc/checker/notify_email.env"""
    
    # Send email
    from_addr = config.get("NOTIFY_EMAIL_FROM", "checker@localhost")
    to_addr = config.get("NOTIFY_EMAIL_TO", "root@localhost")
    
    if send_email(subject, body, from_addr, to_addr):
        print(f"Email notification sent to {to_addr} for {check_name} ({status_name})")
    else:
        sys.exit(1)


if __name__ == "__main__":
    main()