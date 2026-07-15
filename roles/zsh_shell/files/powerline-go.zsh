# SPDX-FileCopyrightText: 2017 Janne Mareike Koschinski
# see /usr/share/doc/powerline-go/copyright
# SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: GPL-3.0-only

function powerline_precmd() {
    PS1="$(/usr/bin/powerline-go -error $? -jobs ${${(%):%j}:-0} -shell zsh)"
}

function install_powerline_precmd() {
    for s in "${precmd_functions[@]}"; do
        if [ "$s" = "powerline_precmd" ]; then
            return
        fi
    done
    precmd_functions+=(powerline_precmd)
}

if [ "$TERM" != "linux" ] && [ -f "/usr/bin/powerline-go" ]; then
    install_powerline_precmd
fi
