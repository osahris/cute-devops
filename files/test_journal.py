#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: EUPL-1.2
import systemd.journal
import time

def main():
    reader = systemd.journal.Reader()
    reader.this_boot()
    reader.add_match(_SYSTEMD_UNIT="deploy@test.service")
    
    # Check last few entries
    reader.seek_tail()
    reader.get_previous()
    
    print("=== Last 3 entries ===")
    for _ in range(3):
        entry = reader.get_previous()
        if entry:
            print(f"Message: {entry.get('MESSAGE', 'N/A')}")
            print(f"Unit state: {entry.get('UNIT_RESULT', 'N/A')}")
            print("---")
    
    # Show key fields available
    if entry:
        print("\nKey fields:", [k for k in entry.keys() if k.startswith('_SYSTEMD') or k == 'MESSAGE'])

if __name__ == "__main__":
    main()