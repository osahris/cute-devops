// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package review

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Scope describes what a review covers.
type Scope struct {
	Branch  string   // the branch being reviewed ("to" side)
	Base    string   // ref the diff is taken against ("from" side)
	TipSHA  string   // resolved tip of Branch at time of scope computation
	BaseSHA string   // resolved tip of Base at time of scope computation
	Diff    string   // symbolic diff range (Base..Branch)
	Commits []Commit // commits in Base..Branch, oldest last (git log order)
	Files   []string // paths changed in Base...Branch
	RawDiff string   // full unified diff, base...branch
	Title   string   // parsed from most-recent [Merge Request] subject; falls back to branch

	// FilePatches: per-file unified diff (git diff base..branch -- <path>).
	// Populated lazily by Render so unused files don't pay the cost.
	FilePatches map[string]string

	// CommitPatches: per-commit `git format-patch --stdout` body keyed by SHA.
	// Lazily populated.
	CommitPatches map[string]string
}

// Commit is one entry in Scope.Commits. Patch is the mbox-style git
// format-patch output for that single commit.
type Commit struct {
	SHA     string
	Short   string
	Subject string
	Patch   string
}

// ScopeFor computes a Scope for the given branch.
// If base is empty, it tries to parse `Base:` from the most-recent
// [Merge Request] commit on the branch; failing that, defaults to "main".
func ScopeFor(branch, base string) (*Scope, error) {
	if branch == "" {
		return nil, fmt.Errorf("scope: branch is required")
	}
	tip, err := gitOut("rev-parse", "--verify", branch)
	if err != nil {
		return nil, fmt.Errorf("scope: %w", err)
	}

	if base == "" {
		base = parseBaseFromBranch(branch)
	}
	if base == "" {
		base = "main"
	}
	baseSHA, err := gitOut("rev-parse", "--verify", base)
	if err != nil {
		return nil, fmt.Errorf("scope: base ref %q not found (override with --base)", base)
	}

	commitsRaw, err := gitOut("log", "--format=%H %s", base+".."+branch)
	if err != nil {
		return nil, fmt.Errorf("scope: %w", err)
	}
	if commitsRaw == "" {
		return nil, fmt.Errorf("scope: no commits in %s..%s", base, branch)
	}
	var commits []Commit
	for _, line := range strings.Split(commitsRaw, "\n") {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		short := parts[0]
		if len(short) > 12 {
			short = short[:12]
		}
		// Fetch the per-commit patch (mbox-style format-patch output).
		patch, _ := gitOut("format-patch", "-1", "--stdout", "--no-signature",
			"--no-color", parts[0])
		commits = append(commits, Commit{
			SHA:     parts[0],
			Short:   short,
			Subject: parts[1],
			Patch:   patch,
		})
	}

	filesRaw, err := gitOut("diff", "--no-color", "--name-only", base+"..."+branch)
	if err != nil {
		return nil, fmt.Errorf("scope: %w", err)
	}
	var files []string
	for _, f := range strings.Split(filesRaw, "\n") {
		if f != "" {
			files = append(files, f)
		}
	}

	// -U2 --inter-hunk-context=0 produces smaller, more digestible hunks:
	// 2 lines of context (instead of git's default 3), and no merging of
	// nearby hunks. A 10-line file with two unrelated 1-line changes is
	// rendered as two small hunks rather than one combined block.
	rawDiff, err := gitOut("diff", "--no-color",
		"-U2", "--inter-hunk-context=0",
		base+"..."+branch)
	if err != nil {
		return nil, fmt.Errorf("scope: %w", err)
	}

	title := findMRTitle(branch)
	if title == "" {
		title = branch
	}

	return &Scope{
		Branch:        branch,
		Base:          base,
		TipSHA:        tip,
		BaseSHA:       baseSHA,
		Diff:          base + ".." + branch,
		Commits:       commits,
		Files:         files,
		RawDiff:       rawDiff,
		Title:         title,
		FilePatches:   map[string]string{},
		CommitPatches: map[string]string{},
	}, nil
}

// FilePatch returns the unified diff for one path in scope, computing it on
// demand and caching.
func (s *Scope) FilePatch(path string) string {
	if p, ok := s.FilePatches[path]; ok {
		return p
	}
	out, err := gitOut("diff", "--no-color",
		"-U2", "--inter-hunk-context=0",
		s.Base+".."+s.Branch, "--", path)
	if err != nil {
		return ""
	}
	if s.FilePatches == nil {
		s.FilePatches = map[string]string{}
	}
	s.FilePatches[path] = out
	return out
}

// CommitPatch returns the `git format-patch --stdout` body for one commit
// in scope, computing on demand and caching.
func (s *Scope) CommitPatch(sha string) string {
	if p, ok := s.CommitPatches[sha]; ok {
		return p
	}
	out, err := gitOut("format-patch", "--stdout", sha+"^.."+sha)
	if err != nil {
		return ""
	}
	if s.CommitPatches == nil {
		s.CommitPatches = map[string]string{}
	}
	s.CommitPatches[sha] = out
	return out
}

// gitOut runs `git` with the given args and returns stdout (trailing newline trimmed).
func gitOut(args ...string) (string, error) {
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

var (
	basePat   = regexp.MustCompile(`(?m)^Base:\s*(\S+)\s*$`)
	mrSubjPat = regexp.MustCompile(`^\[Merge Request\]\s*(.*)$`)
)

func parseBaseFromBranch(branch string) string {
	out, err := gitOut("log", "--format=%H", branch)
	if err != nil {
		return ""
	}
	for _, sha := range strings.Fields(out) {
		msg, err := gitOut("log", "-1", "--format=%B", sha)
		if err != nil {
			continue
		}
		first := strings.SplitN(msg, "\n", 2)[0]
		if !strings.HasPrefix(first, "[Merge Request]") {
			continue
		}
		if m := basePat.FindStringSubmatch(msg); m != nil {
			return m[1]
		}
		return "main"
	}
	return ""
}

func findMRTitle(branch string) string {
	out, err := gitOut("log", "--format=%H %s", branch)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		if m := mrSubjPat.FindStringSubmatch(parts[1]); m != nil {
			return m[1]
		}
	}
	return ""
}
