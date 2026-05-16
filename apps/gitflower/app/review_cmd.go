// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package app

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"gitflower/review"
	"gitflower/tui"
)

func cmdReview(args []string, stdout, stderr io.Writer) int {
	// Subcommand split: `gitflower review merge ...` archives the
	// review note into the branch as an `-s ours` merge.
	if len(args) > 0 && args[0] == "merge" {
		return cmdReviewMerge(args[1:], stdout, stderr)
	}
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "", "also write the review body to this file path (default: notes-only, no on-disk file)")
	baseOverride := fs.String("base", "", "override the base ref (default: last [Review] merge, then main)")
	noTUI := fs.Bool("no-tui", false, "scaffold the review and exit; do not launch the TUI")
	readRate := fs.Float64("read-rate", tui.DefaultReadRate, "assumed reading speed in lines/second")
	notesRef := fs.String("notes-ref", review.DefaultNotesRef, "git notes ref to load/store the review body in")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: gitflower review [-o path] [--base ref] [--notes-ref ref] [--no-tui] [<branch>]")
		fmt.Fprintln(stderr, "       gitflower review merge [--include-file]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Open a review for <branch> (default: current HEAD).")
		fmt.Fprintln(stderr, "Review state lives as a note on refs/notes/review for the branch tip;")
		fmt.Fprintln(stderr, "pass -o to also mirror the body into a file on disk.")
		fmt.Fprintln(stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	branch, err := resolveBranch(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "review: %v\n", err)
		return 1
	}

	reviewer, err := gitOutput("config", "user.email")
	if err != nil || reviewer == "" {
		fmt.Fprintln(stderr, "review: git config user.email is unset")
		return 1
	}

	scope, err := review.ScopeFor(branch, *baseOverride)
	if err != nil {
		fmt.Fprintf(stderr, "review: %v\n", err)
		return 1
	}

	tipSHA := scope.TipSHA

	// Optional file mirror.
	path := *out
	if path != "" && !strings.HasPrefix(path, "/") {
		root, _ := gitOutput("rev-parse", "--show-toplevel")
		if root != "" {
			path = root + "/" + path
		}
	}

	// Try loading the existing note first (notes are the source of
	// truth for ongoing review state).
	sess, err := review.LoadFromNote(*notesRef, tipSHA)
	if err != nil {
		fmt.Fprintf(stderr, "review: load note: %v\n", err)
		return 1
	}
	if sess == nil && path != "" && fileExists(path) {
		// Migration path: an on-disk review for this branch tip is
		// loaded into the note on first run.
		sess, err = review.Load(path)
		if err != nil {
			fmt.Fprintf(stderr, "review: load %s: %v\n", path, err)
			return 1
		}
	}
	if sess == nil {
		sess = review.New(*scope, reviewer, path)
	}
	sess.Scope = *scope
	sess.NotesRef = *notesRef
	sess.NotesSHA = tipSHA
	sess.Path = path
	if sess.Reviewer == "" {
		sess.Reviewer = reviewer
	}

	if *noTUI {
		if err := sess.Save(); err != nil {
			fmt.Fprintf(stderr, "review: save: %v\n", err)
			return 1
		}
		printExitHint(stdout, *notesRef, tipSHA, path)
		return 0
	}

	if err := tui.Run(sess, *readRate); err != nil {
		fmt.Fprintf(stderr, "review: tui: %v\n", err)
		return 1
	}
	printExitHint(stdout, *notesRef, tipSHA, path)
	return 0
}

// printExitHint writes the "where did my review go" footer. The
// review is on a git note; users without gitflower can still read
// it with stock git + grep using the printed commands.
func printExitHint(w io.Writer, notesRef, tipSHA, mirrorPath string) {
	fmt.Fprintf(w, "\nreview saved → %s @ %s\n", notesRef, tipSHA[:12])
	fmt.Fprintln(w, "\nView the full review:")
	cmds := review.ViewCommands(notesRef, tipSHA)
	fmt.Fprintf(w, "  %s\n", cmds[0])
	fmt.Fprintln(w, "\nDrop reading/skip bookkeeping:")
	fmt.Fprintf(w, "  %s\n", cmds[1])
	fmt.Fprintln(w, "\nJust the reactions, with surrounding context:")
	fmt.Fprintf(w, "  %s\n", cmds[2])
	if mirrorPath != "" {
		fmt.Fprintf(w, "\nFile mirror: %s\n", mirrorPath)
	}
}

func resolveBranch(arg string) (string, error) {
	if arg != "" {
		if _, err := gitOutput("rev-parse", "--verify", arg); err != nil {
			return "", fmt.Errorf("branch %q not found", arg)
		}
		return arg, nil
	}
	b, err := gitOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if b == "HEAD" {
		return "", fmt.Errorf("HEAD is detached; pass an explicit <branch>")
	}
	return b, nil
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
