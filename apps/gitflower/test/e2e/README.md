<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-License-Identifier: EUPL-1.2
-->

# gitflower e2e tests

Two ways of exercising the binary against a deterministic fixture repo.

| Runner | Path | What it checks |
|---|---|---|
| `go test ./test/e2e/` | PTY-driven, spawns the real binary | full interactive flow: build → setup → spawn under PTY → walk + comment + verdict → save → diff a real `.review` file |
| `./smoke.sh` | scripted | format pipeline: build → setup → `gitflower review --no-tui` → normalised diff vs golden |

Both rely on `setup.sh`, which rebuilds a small, deterministic git repo at `/tmp/gitflower-e2e-repo`: fixed author identities and commit dates → stable SHAs.

## Running

From `apps/gitflower/`:

```bash
make test         # unit + TUI integration + PTY e2e (all Go tests)
make e2e-format   # smoke.sh
make e2e          # both of the above
```

Or directly:

```bash
go test ./test/e2e/ -v
./test/e2e/smoke.sh
./test/e2e/smoke.sh --update   # rewrite the golden after intentional changes
```

## The PTY test (`e2e_test.go`)

Uses `github.com/creack/pty` to spawn `gitflower review --base main feature` on a real PTY against the fixture repo, then drives this scripted sequence:

1. wait 800 ms for the first frame
2. `Space` — section mode drills into Changes' first unread hunk
3. `>` — cycle verdict to `requested-changes`
4. `c` — open the inline comment editor
5. type `Inline comment from PTY test.`
6. `Alt+Enter` — submit (`\x1b\r` on the wire)
7. `s` — explicit save
8. `q` — quit

The produced `reviews/*.review` is read back and asserted to contain the expected sections (`# Review`, `## Sources`, `## Verdicts`, the new verdict, both file diffs, the comment).

## The smoke check (`smoke.sh`)

Runs the binary with `--no-tui`, skipping the TUI entirely. Useful for nailing down format regressions in CI where you don't want to depend on terminal sizing. Diffs against `expected/smoke.review` after a `sed`-based normaliser replaces dates and SHAs with placeholders.

Update the golden after an intentional format change:

```bash
./smoke.sh --update
```

## Layout

```
test/e2e/
├── setup.sh            # rebuilds /tmp/gitflower-e2e-repo
├── e2e_test.go         # PTY-driven Go test
├── smoke.sh            # --no-tui golden diff
└── expected/
    └── smoke.review    # normalised golden
```
