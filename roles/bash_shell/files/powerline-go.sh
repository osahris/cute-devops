# SPDX-FileCopyrightText: 2017 Janne Mareike Koschinski
# see /usr/share/doc/powerline-go/copyright
# SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: AGPL-3.0-or-later

function _update_ps1() {
    PS1="$(/usr/bin/powerline-go -error $? -jobs $(jobs -p | wc -l) -shell bash -hostname-only-if-ssh)"
}

if [ "$TERM" != "linux" ] && [ -f "/usr/bin/powerline-go" ]; then
    PROMPT_COMMAND="_update_ps1; $PROMPT_COMMAND"
fi
