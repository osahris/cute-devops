// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package review_test

import (
	"strings"
	"testing"

	"gitflower/review"
)

// TestParseInitializesSkippedMap reproduces the user's "assignment
// to entry in nil map" panic from session.go:305. Symptom: load a
// session from a note (via Parse), call MarkSkipped on it — boom,
// because Parse forgot to allocate s.skipped while New did. Fix
// initializes the map and the mutator guards against nil.
func TestParseInitializesSkippedMap(t *testing.T) {
	body := `# Review

## Sources

- From: ` + "`main`" + ` at ` + "``" + `
- To: ` + "`feat`" + ` at ` + "``" + `
- Diff: ` + "`main..feat`" + `
- Commits: 0

## Verdicts

### Verdict: open (From: a <a@e>, 2026-05-16T00:00:00Z)

# General Issues

# Changes

# Commits
`
	s, err := review.Parse(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Before the fix this panicked with "assignment to entry in nil map".
	s.MarkSkipped(review.Anchor("foo.txt:1"))
	if !s.IsSkipped(review.Anchor("foo.txt:1")) {
		t.Errorf("MarkSkipped did not stick")
	}
	// Same goes for MarkRead and SetMarker — guard everything.
	s.MarkRead(review.Anchor("foo.txt:2"))
	if !s.IsRead(review.Anchor("foo.txt:2")) {
		t.Errorf("MarkRead did not stick")
	}
	s.SetMarker(review.Anchor("foo.txt:3"), review.MarkerGood)
	if s.Marker(review.Anchor("foo.txt:3")) != review.MarkerGood {
		t.Errorf("SetMarker did not stick")
	}
}

// TestLineNumbersInDiff verifies the new-style gutter:
//
//	add line:  "> <new>     +content"
//	del line:  "> <old>     -content"
//	ctx line:  "> <new> <old> :content"
//
// All numbers come BEFORE the sign character. Context carries two
// numbers (new and old) so a reader can recover the position even
// when the two sides diverge.
func TestLineNumbersInDiff(t *testing.T) {
	// Synthesise a patch with all three line kinds — one delete, one
	// add, two context lines that straddle the change.
	patch := `diff --git a/x.txt b/x.txt
index 1111111..2222222 100644
--- a/x.txt
+++ b/x.txt
@@ -1,3 +1,3 @@
 alpha
-beta
+BETA
 gamma`
	scope := review.Scope{
		Branch:  "feat",
		Base:    "main",
		TipSHA:  "aaaa1111aaaa",
		BaseSHA: "bbbb2222bbbb",
		Diff:    "main..feat",
		Title:   "feat",
		Files:   []string{"x.txt"},
		RawDiff: patch,
		FilePatches: map[string]string{
			"x.txt": patch,
		},
		CommitPatches: map[string]string{},
	}
	s := review.New(scope, "alice@example.com", "")
	s.AddVerdict(review.VerdictEvent{State: review.VerdictOpen})

	out := review.Render(s)

	for _, want := range []string{
		"> 1 1 :alpha",   // context, new=1 old=1
		"> 2 -beta",      // delete, old=2
		"> 2 +BETA",      // add, new=2
		"> 3 3 :gamma",   // context, new=3 old=3
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered diff missing %q\n--- BEGIN ---\n%s\n--- END ---",
				want, out)
		}
	}

	// Round-trip: Parse should still find anchors correctly. Stick a
	// comment on the added BETA line and make sure it lands at the
	// right anchor after Render→Parse.
	s.AddComment(review.Comment{
		Anchor: review.Anchor("x.txt:2"),
		Text:   "shouted",
		Kind:   review.KindComment,
	})
	out = review.Render(s)
	parsed, err := review.Parse(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := parsed.Comments(); len(got) != 1 {
		t.Fatalf("expected 1 comment after roundtrip, got %d", len(got))
	}
	if got := parsed.Comments()[0].Anchor; got != "x.txt:2" {
		t.Errorf("comment anchor lost in roundtrip: got %q want x.txt:2", got)
	}
}
