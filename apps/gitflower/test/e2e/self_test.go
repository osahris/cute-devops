// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package e2e_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

const selfRepoPath = "/tmp/gitflower-self-test-repo"

// TestSpaceWalkOnSelfRepo clones the current worktree (via setup-self.sh)
// and drives Space repeatedly against `main..experiments/stack-review`
// (a real 27-file / 24-commit diff). The test is "this should make
// forward progress" — every Space must move the cursor or the viewport.
// If 60 presses with read-tick flushes between them don't reach the
// verdict editor, the walk is stuck and we fail.
func TestSpaceWalkOnSelfRepo(t *testing.T) {
	t.Parallel()

	gitflowerBin := buildBinary(t)
	repo := buildSelfRepo(t)

	cmd := exec.Command(gitflowerBin,
		"review",
		"--base", "main",
		"--read-delay", "10ms",
		"experiments/stack-review",
	)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
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

	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 120}); err != nil {
		t.Fatalf("pty.Setsize: %v", err)
	}

	var captured bytes.Buffer
	go io.Copy(&captured, ptmx)

	// Wait for first frame.
	time.Sleep(1200 * time.Millisecond)

	// Spam Space. With a 10ms read delay, each fully-displayed hunk
	// becomes read almost immediately; subsequent Spaces should advance.
	// Large hunks get scrolled within first (paged); once fully visible
	// the timer fires and the next Space jumps to the next unread hunk.
	const maxPresses = 200
	progressFloor := 0
	for i := 0; i < maxPresses; i++ {
		if _, err := ptmx.WriteString(" "); err != nil {
			t.Fatalf("write space %d: %v", i, err)
		}
		time.Sleep(150 * time.Millisecond)
		// Look for the "all read — record your verdict" status line to
		// know when the walk is done.
		if bytes.Contains(captured.Bytes()[progressFloor:], []byte("record your verdict")) {
			break
		}
		progressFloor = captured.Len()
	}

	// At this point we should either be at the verdict editor or very
	// close to it. Quit and check the .review.
	_, _ = ptmx.WriteString("\x1b") // Esc: cancel the verdict editor if open
	time.Sleep(150 * time.Millisecond)
	_, _ = ptmx.WriteString("s") // save
	time.Sleep(200 * time.Millisecond)
	_, _ = ptmx.WriteString("q")
	time.Sleep(150 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := waitCmd(ctx, cmd); err != nil {
		t.Fatalf("gitflower didn't exit: %v\n--- last 2KiB ---\n%s", err, tail(captured.Bytes(), 2048))
	}

	matches, _ := filepath.Glob(filepath.Join(repo, "reviews", "*.review"))
	if len(matches) == 0 {
		t.Fatalf("no .review produced")
	}
	body, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Sanity: the file must contain at least one Verdict, all four major
	// sections, and at least one ReadStart (proving the walk actually
	// scrolled through content).
	for _, want := range []string{
		"# Review\n",
		"## Verdicts\n",
		"# Changes\n",
		"# Commits\n",
		"## Changes in `apps/gitflower/",
	} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("self-review missing %q", want)
		}
	}
	if !bytes.Contains(body, []byte("### ReadStart")) {
		t.Errorf("no ReadStart event in self-review — Space walk made no progress")
	}

	// Count how many distinct hunks got read markers, only counting
	// un-quoted lines so spec-file content that mentions ReadStart isn't
	// double-counted.
	reads := 0
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "### ReadStart ") {
			reads++
		}
	}
	totalHunks := 0
	inChanges := false
	for _, line := range strings.Split(string(body), "\n") {
		switch {
		case strings.HasPrefix(line, "# Changes"):
			inChanges = true
		case strings.HasPrefix(line, "# Commits"):
			inChanges = false
		case inChanges && strings.HasPrefix(line, "> @@"):
			totalHunks++
		}
	}
	t.Logf("self-review: %d/%d hunks marked read after %d Space presses (file: %s)",
		reads, totalHunks, maxPresses, matches[0])
	if totalHunks == 0 {
		t.Fatalf("no hunks parsed from # Changes — fixture is broken")
	}
	// Require at least 70% coverage so we catch the regression (current
	// run produced only ~3% — 4 of ~130 hunks).
	min := totalHunks * 70 / 100
	if reads < min {
		t.Errorf("Space walk stuck: %d/%d hunks read (want at least %d). Last 1.5KiB of TTY:\n%s",
			reads, totalHunks, min, tail(captured.Bytes(), 1536))
	}
}

func buildSelfRepo(t *testing.T) string {
	t.Helper()
	setup := filepath.Join(mustPkgDir(t), "test", "e2e", "setup-self.sh")
	cmd := exec.Command(setup, selfRepoPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("setup-self.sh: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}
