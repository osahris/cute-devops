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
		"--read-delay", "1ms", // minimum, realistic
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
	// After Changes is exhausted, we transition through Commits and
	// finally land in the verdict editor.
	const maxPresses = 300
	pressesUsed := maxPresses
	for i := 0; i < maxPresses; i++ {
		if _, err := ptmx.WriteString(" "); err != nil {
			t.Fatalf("write space %d: %v", i, err)
		}
		time.Sleep(80 * time.Millisecond)
		// Don't try to early-exit by matching screen text: any phrase
		// distinctive enough to flag the verdict editor (e.g. "📝 Verdict
		// summary" or "record your verdict") also appears verbatim in the
		// diff content being reviewed, so the moment those hunks scroll
		// into view we'd false-positive out of the loop. Just spend the
		// whole budget; the walk completes well within it.
	}
	t.Logf("used %d/%d Space presses", pressesUsed, maxPresses)
	// Wait long enough for the autosave debounce + final renders.
	time.Sleep(3 * time.Second)

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
	// Aim for 100% coverage. With --read-delay 1ms and a 30ms inter-
	// Space gap, ticks reliably fire between presses on a quiet machine.
	if reads < totalHunks {
		t.Errorf("Space walk incomplete: %d/%d hunks read. Last 1.5KiB of TTY:\n%s",
			reads, totalHunks, tail(captured.Bytes(), 1536))
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
