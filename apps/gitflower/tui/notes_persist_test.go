// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitflower/review"
)

// TestNotesBackedCommentPersists drives the TUI with a notes-backed
// session in a real git repo. It mirrors what the binary does:
//
//  1. ScopeFor / new session with NotesRef + NotesSHA + Scope set
//  2. user adds a comment via `c` + body + Alt+Enter
//  3. submit calls m.sess.Save() → must write to the notes ref
//  4. assert the note body on refs/notes/review for HEAD contains
//     the comment text
//
// Reproduces the user's report: "I put in a comment and it was not
// there anymore on restart."
func TestNotesBackedCommentPersists(t *testing.T) {
	repo := miniRepo(t)
	chdir(t, repo)

	scope, err := review.ScopeFor("feat", "")
	if err != nil {
		t.Fatalf("ScopeFor: %v", err)
	}
	tipSHA := scope.TipSHA

	sess := review.New(*scope, "tester@example.com", "")
	sess.NotesRef = review.DefaultNotesRef
	sess.NotesSHA = tipSHA

	tmp := t.TempDir()
	m := newModel(sess, tmp, 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Drill in to the first hunk so the comment has an anchor.
	m = key(t, m, ' ', " ")
	if m.mode != modeDiff {
		t.Fatalf("expected modeDiff after Space, got %v", m.mode)
	}

	// Open the comment editor, type a marker, submit with Alt+Enter.
	m = key(t, m, 'c', "c")
	if m.edit != editComment {
		t.Fatalf("expected editComment after 'c', got %v", m.edit)
	}
	for _, r := range "COMMENT-FROM-TUI" {
		m = key(t, m, r, string(r))
	}
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.edit != editNone {
		t.Fatalf("submit did not close edit mode (m.edit=%v)", m.edit)
	}
	if len(sess.Comments()) != 1 {
		t.Fatalf("expected 1 comment in session, got %d", len(sess.Comments()))
	}

	// Now the critical assertion: the note for HEAD must contain the
	// comment body. If the bug is real, this fails.
	out, err := exec.Command("git", "notes",
		"--ref="+review.DefaultNotesRef, "show", tipSHA).CombinedOutput()
	if err != nil {
		t.Fatalf("git notes show: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "COMMENT-FROM-TUI") {
		t.Errorf("note body missing comment; got:\n%s", out)
	}

	// Restart cycle: load the note back and confirm the comment
	// survives parse. This is what the user sees on the next
	// `gitflower review`.
	sess2, err := review.LoadFromNote(review.DefaultNotesRef, tipSHA)
	if err != nil {
		t.Fatalf("LoadFromNote: %v", err)
	}
	if sess2 == nil {
		t.Fatal("LoadFromNote returned nil — note vanished")
	}
	if len(sess2.Comments()) == 0 {
		t.Fatalf("parsed session has no comments; note body:\n%s", out)
	}
	if got := sess2.Comments()[0].Text; got != "COMMENT-FROM-TUI" {
		t.Errorf("parsed comment text = %q; want %q", got, "COMMENT-FROM-TUI")
	}
}

// --- helpers ---------------------------------------------------------

func miniRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustGit(t, dir, "init", "-q", "-b", "main", ".")
	mustGit(t, dir, "config", "user.email", "t@e")
	mustGit(t, dir, "config", "user.name", "tester")
	writeFile(t, dir, "README", "hello\n")
	mustGit(t, dir, "add", "README")
	mustGit(t, dir, "commit", "-q", "-m", "init")
	mustGit(t, dir, "checkout", "-q", "-b", "feat")
	writeFile(t, dir, "x.txt", "alpha\nbeta\ngamma\n")
	mustGit(t, dir, "add", "x.txt")
	mustGit(t, dir, "commit", "-q", "-m", "add x")
	return dir
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	abs := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
