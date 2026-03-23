# SPDX-FileCopyrightText: 2017 Janne Mareike Koschinski
# see /usr/share/doc/powerline-go/copyright
# SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: AGPL-3.0-or-later

function powerline_precmd() {
    PS1="$(/usr/bin/powerline-go -error $? -jobs ${${(%):%j}:-0} -shell zsh -hostname-only-if-ssh)"
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
