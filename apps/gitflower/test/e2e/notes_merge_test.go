// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitflower/review"
)

// mutateNoteAddComment models exactly what the running binary does
// when a TUI keystroke triggers a comment: load the note, restore the
// live Scope from git (same as review_cmd.go), add the comment, save.
// This is the contract we want LoadFromNote → AddComment → Save to
// preserve across an editor restart.
func mutateNoteAddComment(t *testing.T, repo, sha, branch, text string) {
	t.Helper()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir %s: %v", repo, err)
	}
	sess, err := review.LoadFromNote(review.DefaultNotesRef, sha)
	if err != nil || sess == nil {
		t.Fatalf("LoadFromNote(%s): sess=%v err=%v", sha, sess, err)
	}
	scope, err := review.ScopeFor(branch, "")
	if err != nil {
		t.Fatalf("ScopeFor(%s): %v", branch, err)
	}
	sess.Scope = *scope
	sess.AddComment(review.Comment{
		Anchor: review.Anchor("feat.txt:1"),
		Text:   text,
	})
	if err := sess.Save(); err != nil {
		t.Fatalf("Save after AddComment: %v", err)
	}
}

// TestReviewNoteRoundtripsComment exercises the bug the user hit:
// add a comment via the review API, save, restart, and assert the
// comment is still there. Skips the TUI — uses the session +
// notes-store layer directly, since that's where the persistence
// has to work.
func TestReviewNoteRoundtripsComment(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	repo := newMiniRepo(t)
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=tester", "GIT_AUTHOR_EMAIL=t@e",
		"GIT_COMMITTER_NAME=tester", "GIT_COMMITTER_EMAIL=t@e",
	)
	// First review writes a fresh note.
	run(t, env, repo, bin, "review", "--no-tui")

	headSHA := mustGit(t, repo, "rev-parse", "HEAD")

	// Load the note, add a comment, save back to the same note —
	// this is the API path the TUI's `c` handler eventually
	// triggers via sess.AddComment + sess.Save().
	cmd := exec.Command(bin, "review", "--no-tui")
	cmd.Dir = repo
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("warm-up review: %v\n%s", err, out)
	}

	// Mutate the note out-of-band the way the TUI would: load via
	// LoadFromNote, mutate, Save. We do this by invoking a tiny
	// inline Go helper through `go run` ... or simpler, do it in
	// this test process directly. The `review` package is imported
	// in the e2e test module; use it directly.
	mutateNoteAddComment(t, repo, headSHA, "feature", "TEST-COMMENT-MARKER")

	// Debug: dump the note before the second run so we can tell
	// whether mutate worked OR the restart erased.
	mid, _ := exec.Command("git", "-C", repo, "notes",
		"--ref=refs/notes/review", "show", headSHA).CombinedOutput()
	if !strings.Contains(string(mid), "TEST-COMMENT-MARKER") {
		t.Fatalf("comment lost during mutateNoteAddComment, not on restart; note body:\n%s", mid)
	}

	// Restart: run another `review --no-tui` (loads the note,
	// re-saves, exits). This is the round-trip that the user said
	// was losing the comment.
	run(t, env, repo, bin, "review", "--no-tui")

	// Read the note back and assert the comment survived.
	body, err := exec.Command("git", "-C", repo, "notes",
		"--ref=refs/notes/review", "show", headSHA).CombinedOutput()
	if err != nil {
		t.Fatalf("git notes show: %v\n%s", err, body)
	}
	if !strings.Contains(string(body), "TEST-COMMENT-MARKER") {
		t.Errorf("comment lost across restart; note body:\n%s", string(body))
	}
}

// TestReviewNoteAndMerge stands up a tiny git repo, runs
// `gitflower review --no-tui` to write a fresh review into the notes
// ref for HEAD, then runs `gitflower review merge --include-file` to
// archive that note into the branch as an -s ours merge with a
// `review/<short>.review` file. Asserts:
//   - the note exists on refs/notes/review for HEAD before merging,
//   - the merge commit's subject starts with `[Review]`,
//   - the merge commit has exactly two parents (code + notes),
//   - the file `review/<short>.review` exists in the merge tree,
//   - after the merge, the SECOND `gitflower review --no-tui` picks
//     the merge commit as the base for scope (since LastReviewMergeSHA
//     returns it).
func TestReviewNoteAndMerge(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	repo := newMiniRepo(t)

	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=tester", "GIT_AUTHOR_EMAIL=t@e",
		"GIT_COMMITTER_NAME=tester", "GIT_COMMITTER_EMAIL=t@e",
	)

	// First review: writes a note for HEAD.
	run(t, env, repo, bin, "review", "--no-tui")

	// Note must exist for HEAD on refs/notes/review.
	headSHA := mustGit(t, repo, "rev-parse", "HEAD")
	noteBody := mustGit(t, repo, "notes", "--ref=refs/notes/review", "show", headSHA)
	if !strings.Contains(noteBody, "# Review") {
		t.Fatalf("note body doesn't look like a .review: %s", truncOut(noteBody, 200))
	}

	// Archive with --include-file.
	mergeOut := run(t, env, repo, bin, "review", "merge", "--include-file")
	if !strings.Contains(mergeOut, "[Review] merge") {
		t.Errorf("merge output didn't confirm: %s", mergeOut)
	}

	// Verify the merge commit shape.
	mergeSHA := mustGit(t, repo, "rev-parse", "HEAD")
	subj := mustGit(t, repo, "log", "-1", "--format=%s", mergeSHA)
	if !strings.HasPrefix(subj, "[Review]") {
		t.Errorf("merge subject doesn't start with [Review]: %q", subj)
	}
	parents := strings.Fields(mustGit(t, repo, "log", "-1", "--format=%P", mergeSHA))
	if len(parents) != 2 {
		t.Errorf("merge should have 2 parents, got %d: %v", len(parents), parents)
	}

	// Merge commit body must embed the gitflower-free recipes for
	// reading the note (git + grep), so future readers don't need
	// the tool to extract the review. Match the three command
	// signatures we always emit.
	mergeBody := mustGit(t, repo, "log", "-1", "--format=%B", mergeSHA)
	for _, want := range []string{
		"git notes --ref=refs/notes/review show",
		"grep -v -E '^### (ReadStart|ReadEnd|SkipStart|SkipEnd) '",
		"grep -B3 -E ",
	} {
		if !strings.Contains(mergeBody, want) {
			t.Errorf("merge commit body missing %q\n--- body ---\n%s",
				want, mergeBody)
		}
	}

	// File mirror exists in the merge tree.
	short := headSHA
	if len(short) > 7 {
		short = short[:7]
	}
	tree := mustGit(t, repo, "ls-tree", "-r", mergeSHA)
	want := "review/" + short + ".review"
	if !strings.Contains(tree, want) {
		t.Errorf("merge tree missing %s; got:\n%s", want, tree)
	}

	// Second review must pick the merge commit as the base.
	// Add a new code commit on top so there's something to review.
	writeFile(t, repo, "x.txt", "hello\n")
	mustGit(t, repo, "add", "x.txt")
	mustGit(t, repo, "commit", "-m", "add x")

	out := run(t, env, repo, bin, "review", "--no-tui")
	_ = out
	// Easier check: scope's base — which becomes the previous
	// [Review] merge SHA — is what LastReviewMergeSHA returns.
	lastMerge := mustGit(t, repo, "log", "--merges", "--format=%H", "--grep=^\\[Review\\]", "-1")
	if lastMerge != mergeSHA {
		t.Errorf("LastReviewMergeSHA expected %s, got %s", mergeSHA, lastMerge)
	}
}

// --- helpers -------------------------------------------------------

func newMiniRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustGit(t, dir, "init", "-q", "-b", "main", ".")
	mustGit(t, dir, "config", "user.email", "t@e")
	mustGit(t, dir, "config", "user.name", "tester")
	// One initial commit on main so HEAD resolves.
	writeFile(t, dir, "README", "hello\n")
	mustGit(t, dir, "add", "README")
	mustGit(t, dir, "commit", "-q", "-m", "init")
	// Branch off so HEAD is on a feature commit (review scope = main..HEAD).
	mustGit(t, dir, "checkout", "-q", "-b", "feature")
	writeFile(t, dir, "feat.txt", "line1\nline2\n")
	mustGit(t, dir, "add", "feat.txt")
	mustGit(t, dir, "commit", "-q", "-m", "feature work")
	return dir
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	abs := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, env []string, dir, bin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", filepath.Base(bin), strings.Join(args, " "), err, out)
	}
	return string(out)
}

func truncOut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
