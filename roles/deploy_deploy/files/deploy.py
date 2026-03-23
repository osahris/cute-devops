#!/usr/bin/python3
# SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: AGPL-3.0-or-later
import sys
import subprocess
import json
import time
import random
import re
import signal
from datetime import datetime
from collections import deque

# Global variables for cleanup
proc = None
unit_name = None

def signal_handler(signum, frame):
    """Handle keyboard interrupt gracefully"""
    if proc:
        proc.terminate()
    sys.exit(130)  # Standard exit code for SIGINT
        
# Set up signal handler
signal.signal(signal.SIGINT, signal_handler)

if len(sys.argv) != 2:
    print("Usage: deploy.py <instance>", file=sys.stderr)
    sys.exit(1)
    
instance = sys.argv[1]

# Validate instance name
if not re.match(r'^[a-z0-9.-]+$', instance):
    print(f"Error: Instance name '{instance}' contains invalid characters. Only [a-z0-9.-] are allowed.", file=sys.stderr)
    sys.exit(1)

if len(instance) > 128:
    print(f"Error: Instance name '{instance}' is too long. Maximum 128 characters allowed.", file=sys.stderr)
    sys.exit(1)

unit_name = f"deploy@{instance}.service"

# Buffer for messages until service starts
message_buffer = deque()
service_started = False
is_logging = False
skip_until_start = True  # Skip messages until we see our service start

# Open the logging subprocess once
# Use --lines=0 to start from current messages only
proc = subprocess.Popen(['journalctl', '-f', '-u', unit_name, '--output=json', '--lines=0'],
                       stdout=subprocess.PIPE, text=True)

# Check if service is already running
result = subprocess.run(['systemctl', 'is-active', unit_name], 
                      capture_output=True, text=True)
if result.stdout.strip() == 'active':
    print(f"Waiting for {unit_name} to finish...")
    
    # Read journal until we see the service stop
    for line in proc.stdout:
        try:
            entry = json.loads(line)
            comm = entry.get('_COMM', '')
            unit_field = entry.get('UNIT', entry.get('_SYSTEMD_UNIT', ''))
            code_func = entry.get('CODE_FUNC', '')
            
            # Check for systemd's exit message
            if comm == 'systemd' and unit_field == unit_name and (
                code_func == 'unit_log_success' or 
                code_func == 'unit_log_failure'):
                # Service has stopped
                break
        except json.JSONDecodeError:
            continue
    
    # Wait about a second after service stops
    time.sleep(1.0)

# Start the unit
result = subprocess.run(['systemctl', 'start', unit_name])
if result.returncode != 0:
    proc.terminate()
    sys.exit(1)

# Now process log messages continuously
for line in proc.stdout:
    try:
        entry = json.loads(line)
        timestamp = datetime.fromtimestamp(int(entry.get('__REALTIME_TIMESTAMP', 0)) / 1000000)
        pid = entry.get('_PID', '-')
        comm = entry.get('_COMM', 'deploy')
        message = entry.get('MESSAGE', '')
        unit_field = entry.get('UNIT', entry.get('_SYSTEMD_UNIT', ''))
        code_func = entry.get('CODE_FUNC', '')
        
        # Check for systemd service start message
        if comm == 'systemd' and unit_field == unit_name:
            # Look for job_emit_done_message with 'Started' in message
            if code_func == 'job_emit_start_message' and skip_until_start:
                service_started = True
                is_logging = True
                skip_until_start = False  # Now we can start processing messages
                # Output all buffered messages
                for buffered_msg in message_buffer:
                    print(buffered_msg)
                message_buffer.clear()
            # Check for service stop message
            elif code_func == 'unit_log_success' or code_func == 'unit_log_failure':
                is_logging = False
                # Get exit code
                result = subprocess.run(['systemctl', 'show', unit_name, '--property=ExecMainStatus'], 
                                      capture_output=True, text=True)
                return_code = int(result.stdout.split('=')[1].strip())
                if return_code != 0:
                    print(f"Process failed with exit code: {return_code}")
                
                # Terminate journalctl process and exit with the same code
                proc.terminate()
                sys.exit(return_code)
        
        # Skip messages until we see the service start
        if skip_until_start:
            continue
            
        # Format the message for output
        formatted_msg = f"{timestamp.strftime('%b %d %H:%M:%S')} {comm}[{pid}]: {message}"
        
        if is_logging:
            # Service is running and we're logging, output immediately
            print(formatted_msg)
        elif not service_started:
            # Buffer messages until service starts
            message_buffer.append(formatted_msg)
            
    except json.JSONDecodeError:
        # Log malformed JSON to stderr
        print(f"Warning: Malformed JSON in journalctl output: {line.strip()}", file=sys.stderr)
        continue
