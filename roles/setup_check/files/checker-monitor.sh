#!/bin/bash

# SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: EUPL-1.2

# Checker monitoring dashboard - systemd and system resource monitor

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to get status color
get_status_color() {
    case $1 in
        0) echo -e "${GREEN}OK${NC}" ;;
        1) echo -e "${YELLOW}WARNING${NC}" ;;
        2) echo -e "${RED}CRITICAL${NC}" ;;
        3) echo -e "${BLUE}UNKNOWN${NC}" ;;
        *) echo -e "${BLUE}UNKNOWN${NC}" ;;
    esac
}

# Clear screen
clear

echo "=========================================="
echo "      SYSTEMD & CHECKER MONITOR"
echo "=========================================="
echo "Time: $(date)"
echo ""

# Show system resources first
echo "System Resources:"
echo "----------------"
echo "  Hostname: $(hostname -f)"
echo "  Uptime: $(uptime -p)"
echo "  Load Average: $(uptime | awk -F'load average:' '{print $2}')"
echo "  Memory: $(free -h | awk '/^Mem:/ {print $3 " / " $2 " (" int($3/$2 * 100) "%)"}')"
echo "  Disk /: $(df -h / | awk 'NR==2 {print $3 " / " $2 " (" $5 ")"}')"
echo "  Processes: $(ps aux | wc -l) total, $(ps aux | grep -c checker || echo 0) checker"
echo ""

# Show all systemd units status summary
echo "SystemD Units Summary:"
echo "---------------------"
systemctl list-units --no-pager --no-legend | awk '{print $4}' | sort | uniq -c | awk '{printf "  %-12s %s\n", $2":", $1}'
echo ""

# Show failed units
FAILED_UNITS=$(systemctl list-units --failed --no-pager --no-legend | wc -l)
if [ "$FAILED_UNITS" -gt 0 ]; then
    echo -e "${RED}Failed SystemD Units:${NC}"
    systemctl list-units --failed --no-pager
    echo ""
fi

# Show checker timers
echo "Checker Timers:"
echo "---------------"
systemctl list-timers 'checker-*' --no-pager --no-legend | while read line; do
    echo "  $line"
done || echo "  No active checker timers"
echo ""

# Show checker check results
echo "Checker Status:"
echo "---------------"
printf "%-20s %-10s %-15s %s\n" "CHECK" "STATUS" "LAST RUN" "OUTPUT"
echo "----------------------------------------------------------------------"

for check_dir in /etc/checker/checks/*/; do
    if [ -d "$check_dir" ]; then
        check_name=$(basename "$check_dir")
        run_file="/run/checker/${check_name}.out"
        
        if [ -f "$run_file" ]; then
            # Extract metadata
            exit_code=$(grep "Exit-Code:" "$run_file" 2>/dev/null | cut -d' ' -f2 || echo "3")
            last_run=$(grep "Last-Run:" "$run_file" 2>/dev/null | cut -d' ' -f2 | cut -d'T' -f2 | cut -d'+' -f1 || echo "Never")
            
            # Get first line of output
            output=$(head -n1 "$run_file" 2>/dev/null | cut -c1-35 || echo "No output")
            if [ ${#output} -eq 35 ]; then
                output="${output}..."
            fi
            
            # Format status
            status=$(get_status_color "$exit_code")
            
            # Print row
            printf "%-20s %-18s %-15s %s\n" "$check_name" "$status" "$last_run" "$output"
        else
            printf "%-20s %-18s %-15s %s\n" "$check_name" "$(get_status_color 3)" "Never" "Not run yet"
        fi
    fi
done

echo ""

# Show recent checker service logs
echo "Recent Checker Activity:"
echo "-----------------------"
journalctl -u 'checker-*.service' --since '5 minutes ago' --no-pager | tail -n 5 | cut -c1-80 || echo "  No recent activity"
echo ""

# Show resource usage by checker services
echo "Checker Resource Usage:"
echo "----------------------"
for service in $(systemctl list-units 'checker-*.service' --no-legend | awk '{print $1}'); do
    if systemctl is-active --quiet "$service"; then
        mem=$(systemctl show "$service" -p MemoryCurrent | cut -d= -f2)
        if [ "$mem" != "[not set]" ] && [ "$mem" != "18446744073709551615" ]; then
            mem_mb=$((mem / 1024 / 1024))
            echo "  $service: ${mem_mb}MB"
        fi
    fi
done
echo ""

# Refresh instruction
echo "=========================================="
echo "Press Ctrl+C to exit. Auto-refresh in 10s"
echo "=========================================="