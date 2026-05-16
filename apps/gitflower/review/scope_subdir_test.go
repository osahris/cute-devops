// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package review

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestScopeFilePatchFromSubdir reproduces the bug where running
// gitflower from a repo subdirectory caused FilePatch to silently
// return empty for every file — because pathspecs after `--` are
// resolved relative to cwd, while Scope.Files holds root-relative
// paths from `git diff --name-only`. Result: Render emits the
// `## Changes in <path>` headers but no diffs and no inline events
// (comments, marks, reads), so the user's annotations vanish.
//
// Fix: use the `:(top)` pathspec magic so paths anchor to the repo
// root regardless of cwd.
func TestScopeFilePatchFromSubdir(t *testing.T) {
	dir := t.TempDir()
	mustGitS(t, dir, "init", "-q", "-b", "main", ".")
	mustGitS(t, dir, "config", "user.email", "t@e")
	mustGitS(t, dir, "config", "user.name", "tester")
	writeFileS(t, dir, "README", "hi\n")
	mustGitS(t, dir, "add", "README")
	mustGitS(t, dir, "commit", "-q", "-m", "init")
	mustGitS(t, dir, "checkout", "-q", "-b", "feat")
	// Add a file inside a subdirectory so its repo-relative path
	// differs from any cwd-relative resolution from inside `sub/`.
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFileS(t, dir, "sub/x.txt", "a\nb\nc\n")
	mustGitS(t, dir, "add", "sub/x.txt")
	mustGitS(t, dir, "commit", "-q", "-m", "add sub/x")

	// Chdir into the subdir, the way the user invoked gitflower from
	// `apps/gitflower` rather than the repo root.
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(filepath.Join(dir, "sub")); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	scope, err := ScopeFor("feat", "main")
	if err != nil {
		t.Fatalf("ScopeFor: %v", err)
	}
	if got := scope.Files; len(got) != 1 || got[0] != "sub/x.txt" {
		t.Fatalf("Scope.Files = %v; want [sub/x.txt]", got)
	}

	patch := scope.FilePatch("sub/x.txt")
	if patch == "" {
		t.Fatalf("FilePatch returned empty from subdir; should resolve %q via :(top) pathspec",
			"sub/x.txt")
	}
	if !strings.Contains(patch, "+a") {
		t.Errorf("FilePatch missing diff body:\n%s", patch)
	}
}

func mustGitS(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

func writeFileS(t *testing.T, dir, name, body string) {
	t.Helper()
	abs := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
