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
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "", "output path (default: <repo>/reviews/<to>-<sha>-from-<from>-<sha>.review)")
	baseOverride := fs.String("base", "", "override the base ref (default: parsed from `Base:` line, fallback main)")
	noTUI := fs.Bool("no-tui", false, "scaffold the review file and exit; do not launch the TUI")
	readRate := fs.Float64("read-rate", tui.DefaultReadRate, "assumed reading speed in lines/second; per-hunk read delay = lines / read-rate")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: gitflower review [-o path] [--base ref] [--no-tui] [<branch>]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Open a review for <branch> (default: current HEAD).")
		fmt.Fprintln(stderr, "Writes to reviews/<to>-<sha>-from-<from>-<sha>.review and launches the TUI.")
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

	path := *out
	if path == "" {
		root, err := gitOutput("rev-parse", "--show-toplevel")
		if err != nil {
			fmt.Fprintf(stderr, "review: %v\n", err)
			return 1
		}
		path = review.DefaultPath(root, scope)
	}

	var sess *review.ReviewSession
	if fileExists(path) {
		sess, err = review.Load(path)
		if err != nil {
			fmt.Fprintf(stderr, "review: load %s: %v\n", path, err)
			return 1
		}
		// Refresh scope (commits, files, diff) from current git state.
		sess.Scope = *scope
		sess.Path = path
		if sess.Reviewer == "" {
			sess.Reviewer = reviewer
		}
	} else {
		sess = review.New(*scope, reviewer, path)
	}

	if *noTUI {
		if err := sess.Save(); err != nil {
			fmt.Fprintf(stderr, "review: save: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, path)
		return 0
	}

	if err := tui.Run(sess, *readRate); err != nil {
		fmt.Fprintf(stderr, "review: tui: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, path)
	return 0
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
