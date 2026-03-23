# SPDX-FileCopyrightText: 2017 Janne Mareike Koschinski
# see /usr/share/doc/powerline-go/copyright
# SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: AGPL-3.0-or-later

function fish_prompt
    set duration (math -s6 "$CMD_DURATION / 1000")
    /usr/bin/powerline-go -error $status -jobs (count (jobs -p)) -duration $duration -shell bare -hostname-only-if-ssh
end
