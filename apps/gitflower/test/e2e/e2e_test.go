// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

// Package e2e drives the built gitflower binary through a real PTY against
// the constructed fixture repo, asserts on the .review file the TUI writes,
// and shuts the program down with `q`.
//
// Compared to tui/integration_test.go (which drives the bubbletea Model
// in-process), this test exercises the whole binary, the terminal stack,
// the auto-save plumbing, and the on-disk format end-to-end.
package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

const (
	defaultRepoPath = "/tmp/gitflower-e2e-repo"
	defaultStepGap  = 250 * time.Millisecond // breathing room between keystrokes
	defaultExitWait = 3 * time.Second
)

// TestReviewViaPTY: build the binary, rebuild the test repo via setup.sh,
// spawn `gitflower review` under a real PTY, walk through a small scripted
// scenario, then verify the .review file the TUI auto-saved.
func TestReviewViaPTY(t *testing.T) {
	t.Parallel()

	gitflowerBin := buildBinary(t)
	repo := buildRepo(t)

	// Configure git inside the test repo (the binary reads user.email).
	gitCmd(t, repo, "config", "user.email", "reviewer@example.com")

	cmd := exec.Command(gitflowerBin,
		"review",
		"--base", "main",
		"--read-delay", "100ms",
		"feature",
	)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		// Predictable rendering: no Windows-like quirks; ensure a TERM.
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	defer ptmx.Close()
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// Set a terminal size so bubbletea's WindowSizeMsg picks it up.
	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 120}); err != nil {
		t.Fatalf("pty.Setsize: %v", err)
	}

	// Drain output in the background so the PTY never blocks the child.
	var captured bytes.Buffer
	doneRead := make(chan struct{})
	go func() {
		_, _ = io.Copy(&captured, ptmx)
		close(doneRead)
	}()

	// Sequence:
	//   1.  Space      — section mode → drill into Changes' first unread hunk
	//   2.  >          — cycle verdict to requested-changes
	//   3.  c          — open inline comment editor
	//   4.  "Inline comment from PTY test."
	//   5.  Alt+Enter  — submit comment
	//   6.  s          — explicit save
	//   7.  q          — quit
	steps := []sendStep{
		// Wait for the first frame.
		{wait: 800 * time.Millisecond},
		{keys: " ", wait: defaultStepGap},
		{keys: ">", wait: defaultStepGap},
		{keys: "c", wait: defaultStepGap},
		{keys: "Inline comment from PTY test.", wait: defaultStepGap},
		{keys: "\x1b\r", wait: defaultStepGap}, // Alt+Enter → submit
		{keys: "s", wait: defaultStepGap},
		{keys: "q", wait: 100 * time.Millisecond},
	}
	for i, s := range steps {
		if s.keys != "" {
			if _, err := ptmx.WriteString(s.keys); err != nil {
				t.Fatalf("step %d: write %q: %v", i, s.keys, err)
			}
		}
		time.Sleep(s.wait)
	}

	// Wait for the process to exit (its post-quit cleanup is fast).
	ctx, cancel := context.WithTimeout(context.Background(), defaultExitWait)
	defer cancel()
	if err := waitCmd(ctx, cmd); err != nil {
		t.Fatalf("gitflower didn't exit: %v\n--- last 2KiB of PTY output ---\n%s",
			err, tail(captured.Bytes(), 2048))
	}

	// Locate the produced .review file.
	matches, _ := filepath.Glob(filepath.Join(repo, "reviews", "*.review"))
	if len(matches) == 0 {
		t.Fatalf("no .review file produced in %s/reviews\n--- PTY output ---\n%s",
			repo, tail(captured.Bytes(), 4096))
	}
	produced, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read %s: %v", matches[0], err)
	}

	for _, want := range []string{
		"# Review\n",
		"## Sources\n",
		"- From: `main`",
		"- To: `feature`",
		"## Verdicts\n",
		"### Verdict: requested-changes (From: reviewer <reviewer@example.com>",
		"# General Issues\n",
		"# Changes\n",
		"## Changes in `greet.go`",
		"## Changes in `greet_test.go`",
		"# Commits\n",
		"### Comment (From: reviewer <reviewer@example.com>",
		"Inline comment from PTY test.",
	} {
		if !strings.Contains(string(produced), want) {
			t.Errorf("produced .review missing %q\n--- file ---\n%s\n--- end ---",
				want, produced)
		}
	}
}

// ---------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------

type sendStep struct {
	keys string
	wait time.Duration
}

func buildBinary(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "gitflower")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = mustPkgDir(t)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build: %v", err)
	}
	return bin
}

func buildRepo(t *testing.T) string {
	t.Helper()
	setup := filepath.Join(mustPkgDir(t), "test", "e2e", "setup.sh")
	cmd := exec.Command(setup, defaultRepoPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("setup.sh: %v\n%s", err, out)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		path = defaultRepoPath
	}
	return path
}

// mustPkgDir returns the absolute path to apps/gitflower (the Go module root).
func mustPkgDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// We're running from apps/gitflower/test/e2e — go up two.
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// waitCmd waits for cmd to exit, sending SIGTERM if ctx expires.
func waitCmd(ctx context.Context, cmd *exec.Cmd) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case err := <-done:
			return err
		case <-time.After(500 * time.Millisecond):
			_ = cmd.Process.Kill()
			return fmt.Errorf("timed out waiting for exit")
		}
	}
}

// tail returns the last n bytes of b for diagnostic output, with control
// chars rendered visibly.
var ctrlRe = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

func tail(b []byte, n int) string {
	if len(b) > n {
		b = b[len(b)-n:]
	}
	// Strip ANSI escapes so the diagnostic is human-readable.
	return ctrlRe.ReplaceAllString(string(b), "")
}
