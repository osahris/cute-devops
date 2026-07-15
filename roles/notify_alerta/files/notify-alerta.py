#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2024-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-License-Identifier: EUPL-1.2

import os
import sys
import json
import urllib.request
import urllib.error


def load_config():
    """Load configuration from /etc/checker/notify_alerta.env."""
    config = {}
    config_file = "/etc/checker/notify_alerta.env"
    
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


def map_severity(exit_code):
    """Map exit code to Alerta severity."""
    severity_map = {
        "0": "ok",
        "1": "warning",
        "2": "critical",
        "3": "unknown"
    }
    return severity_map.get(exit_code, "debug")


def send_to_alerta(config, hostname, check_id, exit_code, text):
    """Send alert to Alerta."""
    severity = map_severity(exit_code)
    
    # Build alert data
    alert_data = {
        "text": text,
        "resource": hostname,
        "event": check_id,
        "environment": config.get("ALERTA_ENVIRONMENT", "production"),
        "severity": severity,
        "value": exit_code,
        "service": [hostname],
        "origin": "checker",
        "type": "checkerCheck"
    }
    
    # Prepare request
    headers = {
        "Content-Type": "application/json"
    }
    
    # Add API key if configured
    api_key = config.get("ALERTA_API_KEY")
    if api_key:
        headers["Authorization"] = f"Key {api_key}"
    
    # Send request
    url = config["ALERTA_API_ALERT_URL"]
    data = json.dumps(alert_data).encode('utf-8')
    
    try:
        req = urllib.request.Request(url, data=data, headers=headers)
        with urllib.request.urlopen(req) as response:
            result = response.read().decode('utf-8')
            return True, result
    except urllib.error.HTTPError as e:
        error_body = e.read().decode('utf-8') if e.fp else "No error body"
        return False, f"HTTP {e.code}: {error_body}"
    except Exception as e:
        return False, str(e)


def main():
    """Main function."""
    # Load configuration
    config = load_config()
    
    # Parse arguments
    if len(sys.argv) < 4:
        print("Usage: notify-alerta.py <hostname> <check_id> <exit_code>", file=sys.stderr)
        sys.exit(1)
    
    hostname = sys.argv[1]
    check_id = sys.argv[2]
    exit_code = sys.argv[3]
    
    # Read check output from stdin
    text = sys.stdin.read()
    
    # Send to Alerta
    success, result = send_to_alerta(config, hostname, check_id, exit_code, text)
    
    if success:
        print(result)
        sys.exit(0)
    else:
        print(f"Failed to send to Alerta: {result}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()