// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package app

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

func cmdMR(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printMRUsage(stderr)
		return 1
	}
	switch args[0] {
	case "list":
		return cmdMRList(args[1:], stdout, stderr)
	case "-h", "--help":
		printMRUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "mr: unknown subcommand %q\n\n", args[0])
		printMRUsage(stderr)
		return 1
	}
}

func printMRUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gitflower mr <subcommand>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  list     List active merge requests (refs/heads/mr/*)")
}

func cmdMRList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mr list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	archived := fs.Bool("archived", false, "list archived MRs instead (refs/heads/archive/mr/*)")
	all := fs.Bool("all", false, "list both active and archived MRs")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var refs []string
	switch {
	case *all:
		refs = []string{"refs/heads/mr/", "refs/heads/archive/mr/"}
	case *archived:
		refs = []string{"refs/heads/archive/mr/"}
	default:
		refs = []string{"refs/heads/mr/"}
	}

	gitArgs := append([]string{"for-each-ref", "--format=%(refname:short)|%(subject)"}, refs...)
	out, err := gitOutput(gitArgs...)
	if err != nil {
		fmt.Fprintf(stderr, "mr list: %v\n", err)
		return 1
	}
	if out == "" {
		return 0
	}

	// Compute padding for nicer alignment.
	pad := 0
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) < 2 {
			continue
		}
		if n := len(parts[0]); n > pad {
			pad = n
		}
	}
	if pad > 60 {
		pad = 60
	}

	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) < 2 {
			continue
		}
		fmt.Fprintf(stdout, "%-*s  %s\n", pad, parts[0], parts[1])
	}
	return 0
}
