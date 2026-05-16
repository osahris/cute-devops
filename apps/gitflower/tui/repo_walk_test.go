// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitflower/review"
)

// TestSpaceWalkOnThisRepo drives the Space/PgDn walk against the
// current worktree's branch (main..HEAD) entirely in-process. It's a
// repro target for "Space walk gets stuck when reviewing this repo".
// When this passes, walking the live repo can't deadlock; when it
// fails, the failure output tells us exactly which file/hunk the walk
// stalled on.
func TestSpaceWalkOnThisRepo(t *testing.T) {
	if os.Getenv("CI") == "" && testing.Short() {
		t.Skip("set CI=1 or run without -short to exercise the live repo walk")
	}
	root := findRepoRoot(t)
	if root == "" {
		t.Skip("not inside a git repo")
	}
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir to repo root: %v", err)
	}
	scope, err := review.ScopeFor("HEAD", "main")
	if err != nil {
		t.Skipf("ScopeFor failed (no main branch?): %v", err)
	}
	if len(scope.Files) == 0 {
		t.Skip("HEAD has no diff against main")
	}
	t.Logf("scope: %d files, %d commits", len(scope.Files), len(scope.Commits))

	tmp := t.TempDir()
	sess := review.New(*scope, "reviewer@example.com", filepath.Join(tmp, "test.review"))
	m := newModel(sess, root, 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	const maxSteps = 5000
	stuck := 0
	for i := 0; i < maxSteps; i++ {
		for anchor := range m.pendingLines {
			next, _ := m.Update(delayedReadMsg{line: anchor})
			m = next.(*model)
		}
		if m.edit == editSummary {
			t.Logf("reached verdict editor after %d step(s)", i)
			break
		}
		before := stateSig(m)
		var msg tea.Msg
		if m.atEOF || m.mode == modeTree || !m.fileHasUnread(m.fileIdx) {
			msg = tea.KeyPressMsg{Code: ' ', Text: " "}
		} else {
			msg = tea.KeyPressMsg{Code: tea.KeyPgDown}
		}
		m = step(t, m, msg)
		if stateSig(m) == before {
			stuck++
			if stuck > 4 {
				path := ""
				if f := m.currentFile(); f != nil {
					path = f.Path
				}
				var unreadByFile strings.Builder
				for i, f := range m.files {
					if !m.fileHasUnread(i) {
						continue
					}
					read, total := 0, 0
					for _, h := range f.Hunks {
						total++
						a := review.HunkAnchor(f.Path, h.NewStart, h.NewLines)
						if m.sess.IsRead(a) {
							read++
						}
					}
					fmt.Fprintf(&unreadByFile, "  [%d] %s  %d/%d read\n", i, f.Path, read, total)
				}
				t.Fatalf("walk stuck after %d step(s) at %s (path=%s)\nunread files remaining:\n%s",
					i, before, path, unreadByFile.String())
			}
		} else {
			stuck = 0
		}
	}

	// Verify every (real) file's reviewable lines got marked read.
	type miss struct {
		path string
		read int
		all  int
	}
	var misses []miss
	for fi, f := range m.files {
		if strings.HasPrefix(f.Path, "commit:") {
			continue
		}
		r, total := m.fileLineCounts(fi)
		if r != total {
			misses = append(misses, miss{f.Path, r, total})
		}
	}
	if len(misses) > 0 {
		var sb strings.Builder
		for _, mi := range misses {
			fmt.Fprintf(&sb, "  %-50s %d/%d lines read\n", mi.path, mi.read, mi.all)
		}
		t.Errorf("walk left lines unread:\n%s", sb.String())
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
