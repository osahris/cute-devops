# SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-FileCopyrightText: 2017 Janne Mareike Koschinski
# SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
# SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
#
# SPDX-License-Identifier: GPL-3.0-only

function _update_ps1() {
    PS1="$(/usr/bin/powerline-go -error $? -jobs $(jobs -p | wc -l) -shell bash)"
}

if [ "$TERM" != "linux" ] && [ -f "/usr/bin/powerline-go" ]; then
    PROMPT_COMMAND="_update_ps1; $PROMPT_COMMAND"
fi
