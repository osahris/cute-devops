# SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-FileCopyrightText: 2017 Janne Mareike Koschinski
# SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
# SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
#
# SPDX-License-Identifier: GPL-3.0-only

function fish_prompt
    set duration (math -s6 "$CMD_DURATION / 1000")
    /usr/bin/powerline-go -error $status -jobs (count (jobs -p)) -duration $duration -shell bare
end
