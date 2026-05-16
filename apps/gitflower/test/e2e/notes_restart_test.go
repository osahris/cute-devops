// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package e2e_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestReviewRestartShowsComment is the user's exact reported scenario:
// start `gitflower review` (notes-only, no -o flag), type a comment,
// quit, restart, and assert the comment still shows up.
//
// "Should be saved immediately on submission and when I exit the
// program." If a comment vanishes between sessions, this test fails.
func TestReviewRestartShowsComment(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	// Use a per-test repo path so we don't race the shared
	// /tmp/gitflower-e2e-repo with TestReviewViaPTY.
	repo := buildRepoAt(t, "/tmp/gitflower-e2e-repo-restart")
	gitCmd(t, repo, "config", "user.email", "reviewer@example.com")

	// --- session 1: add a unique comment, then quit -----------------
	{
		cmd := exec.Command(bin, "review",
			"--base", "main",
			"--read-rate", "1000",
			"feature",
		)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
		ptmx, err := pty.Start(cmd)
		if err != nil {
			t.Fatalf("pty.Start session 1: %v", err)
		}
		defer ptmx.Close()
		_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 120})

		var captured bytes.Buffer
		go io.Copy(&captured, ptmx)

		steps := []sendStep{
			{wait: 800 * time.Millisecond},
			{keys: " ", wait: defaultStepGap},                              // drill into first hunk
			{keys: "c", wait: defaultStepGap},                              // open comment editor
			{keys: "SESSION-1-COMMENT-MARKER", wait: defaultStepGap},       // body
			{keys: "\r", wait: defaultStepGap},                             // Enter submits
			{keys: "q", wait: 200 * time.Millisecond},                      // quit (flushes save)
		}
		for i, s := range steps {
			if s.keys != "" {
				if _, err := ptmx.WriteString(s.keys); err != nil {
					t.Fatalf("session 1 step %d: %v", i, err)
				}
			}
			time.Sleep(s.wait)
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultExitWait)
		defer cancel()
		if err := waitCmd(ctx, cmd); err != nil {
			t.Fatalf("session 1 didn't exit: %v\n%s", err, tail(captured.Bytes(), 1024))
		}
	}

	// --- between sessions: confirm the note has the marker ----------
	tipSHA := strings.TrimSpace(runGit(t, repo, "rev-parse", "feature"))
	noteOut := runGit(t, repo, "notes", "--ref=refs/notes/review", "show", tipSHA)
	if !strings.Contains(noteOut, "SESSION-1-COMMENT-MARKER") {
		t.Fatalf("after session 1 quit, note for %s missing comment; got:\n%s",
			tipSHA, noteOut)
	}

	// --- session 2: restart, capture screen, look for the marker ----
	cmd := exec.Command(bin, "review",
		"--base", "main",
		"--read-rate", "1000",
		"feature",
	)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start session 2: %v", err)
	}
	defer ptmx.Close()
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 120})

	var captured bytes.Buffer
	go io.Copy(&captured, ptmx)

	steps := []sendStep{
		{wait: 800 * time.Millisecond},
		{keys: " ", wait: defaultStepGap}, // drill into first hunk (comments render in modeDiff)
		{wait: 400 * time.Millisecond},    // give the model time to repaint
		{keys: "q", wait: 200 * time.Millisecond},
	}
	for i, s := range steps {
		if s.keys != "" {
			if _, err := ptmx.WriteString(s.keys); err != nil {
				t.Fatalf("session 2 step %d: %v", i, err)
			}
		}
		time.Sleep(s.wait)
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultExitWait)
	defer cancel()
	if err := waitCmd(ctx, cmd); err != nil {
		t.Fatalf("session 2 didn't exit: %v\n%s", err, tail(captured.Bytes(), 1024))
	}

	// Note must still be intact (Save on quit must not clobber).
	noteOut = runGit(t, repo, "notes", "--ref=refs/notes/review", "show", tipSHA)
	if !strings.Contains(noteOut, "SESSION-1-COMMENT-MARKER") {
		t.Errorf("after session 2 quit, note lost the comment; got:\n%s", noteOut)
	}

	// And the marker must have appeared on screen during session 2
	// (i.e. the TUI actually rendered the loaded comment).
	screen := tail(captured.Bytes(), 32*1024)
	if !strings.Contains(screen, "SESSION-1-COMMENT-MARKER") {
		t.Errorf("session 2 screen never showed the loaded comment;\n--- screen tail ---\n%s",
			screen)
	}
}
