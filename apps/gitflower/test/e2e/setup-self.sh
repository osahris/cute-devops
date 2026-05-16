#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-License-Identifier: EUPL-1.2
#
# Clones the cute-devops worktree we live in to $1 (default
# /tmp/gitflower-self-test-repo) and sets up `main` to track the
# parent's origin/main so `main..experiments/stack-review` resolves to
# the real diff we want to review. Idempotent: nukes $1 first.
#
# Output: prints the path to the rebuilt clone on stdout.

set -euo pipefail

dest="${1:-/tmp/gitflower-self-test-repo}"
src="${GITFLOWER_SELF_SRC:-/srv/repos/cute-devops.git/treehouses/experiments/stack-review}"

rm -rf "$dest"
git clone --quiet "$src" "$dest"
(
  cd "$dest"
  # Track the parent's origin/main so `main..HEAD` is well-defined.
  git branch --quiet -f main origin/main 2>/dev/null || true
  git config user.email reviewer@example.com
  git config user.name "Reviewer"
)
echo "$dest"
