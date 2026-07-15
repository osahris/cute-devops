#!/usr/bin/env python3

# SPDX-FileCopyrightText: 2024 - 2025 Uniklinik Köln
# SPDX-FileCopyrightText: 2024 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
#
# SPDX-License-Identifier: EUPL-1.2

import os
import sys
import subprocess
import time
import glob
from datetime import datetime


# Colors
class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[0;33m'
    BLUE = '\033[0;34m'
    NC = '\033[0m'  # No Color


def get_status_color(exit_code):
    """Get status color based on exit code."""
    status_map = {
        0: f"{Colors.GREEN}OK{Colors.NC}",
        1: f"{Colors.YELLOW}WARNING{Colors.NC}",
        2: f"{Colors.RED}CRITICAL{Colors.NC}",
        3: f"{Colors.BLUE}UNKNOWN{Colors.NC}"
    }
    return status_map.get(exit_code, f"{Colors.BLUE}UNKNOWN{Colors.NC}")


def run_command(cmd, default=""):
    """Run a shell command and return output."""
    try:
        result = subprocess.run(cmd, shell=True, capture_output=True, text=True)
        return result.stdout.strip() if result.returncode == 0 else default
    except:
        return default


def get_system_resources():
    """Get system resource information."""
    hostname = run_command("hostname -f")
    uptime = run_command("uptime -p")
    load_avg = run_command("uptime | awk -F'load average:' '{print $2}'")
    memory = run_command("free -h | awk '/^Mem:/ {print $3 \" / \" $2 \" (\" int($3/$2 * 100) \"%)\"}'")
    disk = run_command("df -h / | awk 'NR==2 {print $3 \" / \" $2 \" (\" $5 \")\"}'")
    total_procs = run_command("ps aux | wc -l")
    checker_procs = run_command("ps aux | grep -c checker || echo 0")
    
    return {
        'hostname': hostname,
        'uptime': uptime,
        'load_avg': load_avg,
        'memory': memory,
        'disk': disk,
        'total_procs': total_procs,
        'checker_procs': checker_procs
    }


def get_systemd_summary():
    """Get systemd units summary."""
    cmd = "systemctl list-units --no-pager --no-legend | awk '{print $4}' | sort | uniq -c"
    output = run_command(cmd)
    if output:
        lines = output.split('\n')
        summary = []
        for line in lines:
            parts = line.strip().split()
            if len(parts) == 2:
                summary.append(f"  {parts[1]:<12} {parts[0]}")
        return '\n'.join(summary)
    return "  No units found"


def get_failed_units():
    """Get failed systemd units."""
    count = run_command("systemctl list-units --failed --no-pager --no-legend | wc -l", "0")
    if int(count) > 0:
        units = run_command("systemctl list-units --failed --no-pager")
        return f"{Colors.RED}Failed SystemD Units:{Colors.NC}\n{units}"
    return None


def get_checker_timers():
    """Get checker timers status."""
    timers = run_command("systemctl list-timers 'checker-*' --no-pager --no-legend")
    if timers:
        return '\n'.join(f"  {line}" for line in timers.split('\n'))
    return "  No active checker timers"


def get_checker_status():
    """Get checker check results."""
    checks = []
    
    for check_dir in glob.glob('/etc/checker/checks/*/'):
        if os.path.isdir(check_dir):
            check_name = os.path.basename(check_dir.rstrip('/'))
            run_file = f"/run/checker/{check_name}.out"
            
            if os.path.exists(run_file):
                try:
                    with open(run_file, 'r') as f:
                        content = f.read()
                    
                    # Extract metadata
                    exit_code = 3
                    last_run = "Never"
                    for line in content.split('\n'):
                        if line.startswith('Exit-Code:'):
                            exit_code = int(line.split()[1])
                        elif line.startswith('Last-Run:'):
                            timestamp = line.split()[1]
                            # Extract time part
                            if 'T' in timestamp:
                                last_run = timestamp.split('T')[1].split('+')[0]
                    
                    # Get first line of output
                    first_line = content.split('\n')[0] if content else "No output"
                    output = first_line[:35]
                    if len(first_line) > 35:
                        output += "..."
                    
                    status = get_status_color(exit_code)
                    checks.append((check_name, status, last_run, output))
                except Exception:
                    checks.append((check_name, get_status_color(3), "Error", "Failed to read"))
            else:
                checks.append((check_name, get_status_color(3), "Never", "Not run yet"))
    
    return checks


def get_recent_activity():
    """Get recent checker activity from journal."""
    cmd = "journalctl -u 'checker-*.service' --since '5 minutes ago' --no-pager | tail -n 5"
    output = run_command(cmd)
    if output:
        lines = output.split('\n')
        return '\n'.join(line[:80] for line in lines)
    return "  No recent activity"


def get_resource_usage():
    """Get resource usage by checker services."""
    usage = []
    services = run_command("systemctl list-units 'checker-*.service' --no-legend | awk '{print $1}'")
    
    if services:
        for service in services.split('\n'):
            if service.strip():
                active = run_command(f"systemctl is-active {service}")
                if active == "active":
                    mem = run_command(f"systemctl show {service} -p MemoryCurrent | cut -d= -f2")
                    if mem and mem != "[not set]" and mem != "18446744073709551615":
                        try:
                            mem_mb = int(mem) // 1024 // 1024
                            usage.append(f"  {service}: {mem_mb}MB")
                        except ValueError:
                            pass
    
    return '\n'.join(usage) if usage else "  No active checker services"


def clear_screen():
    """Clear the terminal screen."""
    os.system('clear' if os.name == 'posix' else 'cls')


def main():
    """Main monitoring loop."""
    try:
        while True:
            clear_screen()
            
            print("==========================================")
            print("      SYSTEMD & CHECKER MONITOR")
            print("==========================================")
            print(f"Time: {datetime.now().strftime('%c')}")
            print()
            
            # System resources
            resources = get_system_resources()
            print("System Resources:")
            print("----------------")
            print(f"  Hostname: {resources['hostname']}")
            print(f"  Uptime: {resources['uptime']}")
            print(f"  Load Average: {resources['load_avg']}")
            print(f"  Memory: {resources['memory']}")
            print(f"  Disk /: {resources['disk']}")
            print(f"  Processes: {resources['total_procs']} total, {resources['checker_procs']} checker")
            print()
            
            # SystemD units summary
            print("SystemD Units Summary:")
            print("---------------------")
            print(get_systemd_summary())
            print()
            
            # Failed units
            failed = get_failed_units()
            if failed:
                print(failed)
                print()
            
            # Checker timers
            print("Checker Timers:")
            print("---------------")
            print(get_checker_timers())
            print()
            
            # Checker status
            print("Checker Status:")
            print("---------------")
            print(f"{'CHECK':<20} {'STATUS':<10} {'LAST RUN':<15} OUTPUT")
            print("-" * 70)
            
            checks = get_checker_status()
            for check_name, status, last_run, output in checks:
                print(f"{check_name:<20} {status:<18} {last_run:<15} {output}")
            print()
            
            # Recent activity
            print("Recent Checker Activity:")
            print("-----------------------")
            print(get_recent_activity())
            print()
            
            # Resource usage
            print("Checker Resource Usage:")
            print("----------------------")
            print(get_resource_usage())
            print()
            
            # Refresh instruction
            print("==========================================")
            print("Press Ctrl+C to exit. Auto-refresh in 10s")
            print("==========================================")
            
            time.sleep(10)
            
    except KeyboardInterrupt:
        print("\nExiting...")
        sys.exit(0)


if __name__ == "__main__":
    main()