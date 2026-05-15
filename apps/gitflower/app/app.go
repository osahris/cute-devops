// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package app

import (
	"fmt"
	"io"
)

func App(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}
	switch args[0] {
	case "review":
		return cmdReview(args[1:], stdout, stderr)
	case "mr":
		return cmdMR(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "gitflower: unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 1
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "gitflower — git helpers")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  gitflower review [<branch>]   Open a TUI review of <branch>")
	fmt.Fprintln(w, "  gitflower mr list             List active merge requests")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run `gitflower <command> -h` for command-specific flags.")
}
