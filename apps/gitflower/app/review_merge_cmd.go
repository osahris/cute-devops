// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package app

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gitflower/review"
)

// cmdReviewMerge creates a merge commit on the current branch that
// links in the review-notes ref via -s ours. The branch's tree is
// unchanged; the notes objects ride along as the merge commit's
// second-parent reachability. Optional --include-file also writes the
// rendered review body to review/<tip-short>.review in the merge
// commit's tree.
func cmdReviewMerge(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("review merge", flag.ContinueOnError)
	fs.SetOutput(stderr)
	includeFile := fs.Bool("include-file", false, "also write the review body to review/<tip-short>.review in the merge commit's tree")
	notesRef := fs.String("notes-ref", review.DefaultNotesRef, "git notes ref holding the review body")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: gitflower review merge [--include-file] [--notes-ref ref]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Archive the current review-notes ref into HEAD as an -s ours merge")
		fmt.Fprintln(stderr, "commit subjected `[Review] <tip-short>`. The branch's tree is")
		fmt.Fprintln(stderr, "unchanged unless --include-file is passed.")
		fmt.Fprintln(stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	tipSHA, err := gitOutput("rev-parse", "--verify", "HEAD")
	if err != nil {
		fmt.Fprintf(stderr, "review merge: %v\n", err)
		return 1
	}
	notesTip, err := review.NotesRefTip(*notesRef)
	if err != nil {
		fmt.Fprintf(stderr, "review merge: %v\n", err)
		return 1
	}
	if notesTip == "" {
		fmt.Fprintf(stderr, "review merge: notes ref %q is empty — nothing to archive\n", *notesRef)
		return 1
	}

	// Confirm a note exists for HEAD; if not, refuse rather than
	// archive an irrelevant notes ref.
	body, err := review.ReadNote(*notesRef, tipSHA)
	if err != nil {
		fmt.Fprintf(stderr, "review merge: %v\n", err)
		return 1
	}
	if body == "" {
		fmt.Fprintf(stderr, "review merge: no note for HEAD (%s) on %s\n",
			tipSHA[:12], *notesRef)
		return 1
	}

	short := tipSHA
	if len(short) > 7 {
		short = short[:7]
	}
	subject := fmt.Sprintf("[Review] %s", short)

	// Build the merge commit tree.
	mergeTree, err := gitOutput("rev-parse", "HEAD^{tree}")
	if err != nil {
		fmt.Fprintf(stderr, "review merge: %v\n", err)
		return 1
	}

	if *includeFile {
		// Stash the review body in review/<short>.review at the
		// top of the worktree, stage it, then re-build the merge
		// tree from the index. Restore the index/worktree after.
		root, err := gitOutput("rev-parse", "--show-toplevel")
		if err != nil {
			fmt.Fprintf(stderr, "review merge: %v\n", err)
			return 1
		}
		relPath := filepath.Join("review", short+".review")
		abs := filepath.Join(root, relPath)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			fmt.Fprintf(stderr, "review merge: mkdir: %v\n", err)
			return 1
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			fmt.Fprintf(stderr, "review merge: write: %v\n", err)
			return 1
		}
		if _, err := gitOutput("add", "--", relPath); err != nil {
			fmt.Fprintf(stderr, "review merge: git add: %v\n", err)
			return 1
		}
		mergeTree, err = gitOutput("write-tree")
		if err != nil {
			fmt.Fprintf(stderr, "review merge: write-tree: %v\n", err)
			return 1
		}
	}

	// commit-tree with two parents: HEAD (first) and the notes tip
	// (second). First-parent stays on the code line. The commit body
	// embeds the same shell recipes the `gitflower review` exit hint
	// prints, so a future reader can extract the review with stock
	// git + grep — no gitflower required.
	viewCmds := review.ViewCommands(*notesRef, tipSHA)
	commitBody := fmt.Sprintf(
		"Notes-Ref: %s\nNotes-Source-Tip: %s\n\n"+
			"View the full review:\n  %s\n\n"+
			"Drop reading/skip bookkeeping:\n  %s\n\n"+
			"Just the reactions, with surrounding context:\n  %s\n",
		*notesRef, notesTip,
		viewCmds[0], viewCmds[1], viewCmds[2],
	)
	cmd := exec.Command("git", "commit-tree",
		mergeTree, "-p", tipSHA, "-p", notesTip,
		"-m", subject,
		"-m", commitBody,
	)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(stderr, "review merge: commit-tree: %s\n", strings.TrimSpace(string(ee.Stderr)))
		} else {
			fmt.Fprintf(stderr, "review merge: commit-tree: %v\n", err)
		}
		return 1
	}
	newSHA := strings.TrimSpace(string(out))

	// Advance current branch to the new merge commit.
	if _, err := gitOutput("update-ref", "HEAD", newSHA); err != nil {
		fmt.Fprintf(stderr, "review merge: update-ref HEAD: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "[Review] merge %s created (tree=%s, second parent=%s)\n",
		newSHA[:12], mergeTree[:12], notesTip[:12])
	return 0
}
