# SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: AGPL-3.0-or-later

function fish_title
  # Just calculate this once, to save a few cycles when displaying the prompt
  if not set -q __fish_prompt_hostname
    set -g __fish_prompt_hostname (hostname|cut -d . -f 1)
  end

	set -l prefix
  set -l suffix

  switch $USER
  case root toor
    set prefix "$__fish_prompt_hostname:"
    set suffix '#'
  case '*'
    set prefix "$USER@$__fish_prompt_hostname:"
    set suffix '>'
  end

	if [ "$XDG_SESSION_TYPE" != "x11" ]
		echo -n -s "$prefix"
	end

  echo -n -s (prompt_pwd) " $suffix $_"
end
