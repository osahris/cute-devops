// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

// Notes-backed review storage. A reviewer's session for one commit
// lives as the note body for that commit in refs/notes/review (or
// whichever ref the caller specifies). Same .review format as the
// in-tree file path; just stored content-addressed in the object
// store and reachable by commit SHA.

package review

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// DefaultNotesRef is the conventional refs/notes/* ref used for
// review bodies.
const DefaultNotesRef = "refs/notes/review"

// ReadNote returns the note body for `sha` from the named notes ref,
// or "" + nil if no note exists. Any other git failure surfaces as an
// error.
func ReadNote(ref, sha string) (string, error) {
	if ref == "" {
		ref = DefaultNotesRef
	}
	cmd := exec.Command("git", "notes", "--ref="+ref, "show", sha)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		// "no note found" exits non-zero with a specific message;
		// treat that as a clean miss.
		if strings.Contains(stderr.String(), "no note found") ||
			strings.Contains(stderr.String(), "No note found") {
			return "", nil
		}
		return "", fmt.Errorf("git notes show: %v: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// WriteNote stores `body` as the note for `sha` on the named notes
// ref, overwriting any previous note. Uses `git notes add -f -F -` so
// stdin carries the body verbatim (no -m escaping issues).
func WriteNote(ref, sha, body string) error {
	if ref == "" {
		ref = DefaultNotesRef
	}
	cmd := exec.Command("git", "notes", "--ref="+ref, "add", "-f", "-F", "-", sha)
	cmd.Stdin = strings.NewReader(body)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git notes add: %v: %s", err, stderr.String())
	}
	return nil
}

// LoadFromNote reads the note for `sha` on `ref` and parses it as a
// .review session. Returns (nil, nil) if there's no note for the
// commit. Any parse failure or git failure surfaces as an error.
func LoadFromNote(ref, sha string) (*ReviewSession, error) {
	body, err := ReadNote(ref, sha)
	if err != nil {
		return nil, err
	}
	if body == "" {
		return nil, nil
	}
	sess, err := Parse(body)
	if err != nil {
		return nil, err
	}
	sess.NotesRef = ref
	sess.NotesSHA = sha
	return sess, nil
}

// LastReviewMergeSHA returns the SHA of the most recent merge commit
// reachable from `branch` whose subject starts with "[Review]". Used
// as the default base for a new review: everything since the last
// archived review is the new scope. Returns "" with nil error if no
// such merge exists.
func LastReviewMergeSHA(branch string) (string, error) {
	if branch == "" {
		branch = "HEAD"
	}
	cmd := exec.Command("git", "log", "--merges", "--format=%H %s", branch)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git log --merges: %v: %s", err, stderr.String())
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		sp := strings.SplitN(line, " ", 2)
		if len(sp) < 2 {
			continue
		}
		if strings.HasPrefix(sp[1], "[Review]") {
			return sp[0], nil
		}
	}
	return "", nil
}

// ViewCommands returns shell snippets a reader can copy-paste to
// read the review for `sha` on `ref` — three of them, in order of
// increasing terseness:
//
//  1. raw — the full body, exactly as stored.
//  2. quiet — same, minus Read/Skip events (which are TUI bookkeeping
//     and rarely interesting when reading the prose).
//  3. terse — just headings and reactions with the 3 diff lines that
//     precede each, for "what was actually said and where".
//
// Used by the `gitflower review` exit hint and embedded in the
// [Review] merge commit body so the same recipes survive in git
// history without needing the tool to read them.
func ViewCommands(ref, sha string) []string {
	short := sha
	if len(short) > 12 {
		short = short[:12]
	}
	prefix := fmt.Sprintf("git notes --ref=%s show %s", ref, short)
	return []string{
		prefix,
		prefix + ` | grep -v -E '^### (ReadStart|ReadEnd|SkipStart|SkipEnd) '`,
		prefix + ` | grep -B3 -E '^(# |## (Sources|Verdicts|Changes in |Issue |Commit |File )|### (Comment|Question|Like|Dislike|Verdict))'`,
	}
}

// NotesRefTip returns the OID that refs/notes/<ref> currently points
// at, or "" if the ref doesn't exist yet.
func NotesRefTip(ref string) (string, error) {
	if ref == "" {
		ref = DefaultNotesRef
	}
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", ref)
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out)), nil
}
