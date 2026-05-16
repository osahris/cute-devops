// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"testing"

	"gitflower/review"
)

// TestLineReadSyncsToSession confirms that marking a TUI line read
// pushes into sess.read so Save persists it. Reproduces the user's
// report: "the read lines are not imported at all" — root cause was
// that the TUI tracked lineRead in its own map without ever telling
// the session, so the rendered note had zero ReadStart events.
func TestLineReadSyncsToSession(t *testing.T) {
	sess := newSessionWithDiff(t)
	m := newModel(sess, t.TempDir(), 1000.0)

	// The mock diff is a one-hunk, two-line new file. Mark the first
	// `+` line read and verify the session picks it up.
	lk := lineKey{fileIdx: 0, hunkIdx: 0, lineIdx: 0}
	m.markLineRead(lk)

	anchors := sess.ReadAnchors()
	if len(anchors) != 1 {
		t.Fatalf("expected 1 read anchor in session, got %d (%v)", len(anchors), anchors)
	}
	if string(anchors[0]) != "foo.txt:1" {
		t.Errorf("anchor = %q; want foo.txt:1", anchors[0])
	}
}

// TestHydrateRestoresReadLines is the other half of the round-trip:
// a session with persisted reads must populate m.lineRead on startup
// so the TUI shows the prior progress instead of starting blank.
func TestHydrateRestoresReadLines(t *testing.T) {
	sess := newSessionWithDiff(t)

	// Pretend the previous run saved a read on foo.txt:2.
	sess.MarkRead(review.Anchor("foo.txt:2"))

	m := newModel(sess, t.TempDir(), 1000.0)

	want := lineKey{fileIdx: 0, hunkIdx: 0, lineIdx: 1}
	if !m.lineRead[want] {
		t.Errorf("hydrate didn't restore foo.txt:2 to lineRead; got %v",
			m.lineRead)
	}
}

// TestSkipSyncsToSession confirms the same pattern for the skip lane.
func TestSkipSyncsToSession(t *testing.T) {
	sess := newSessionWithDiff(t)
	m := newModel(sess, t.TempDir(), 1000.0)

	lk := lineKey{fileIdx: 0, hunkIdx: 0, lineIdx: 0}
	m.markLineSkipped(lk)

	anchors := sess.SkippedAnchors()
	if len(anchors) != 1 || string(anchors[0]) != "foo.txt:1" {
		t.Errorf("skipped anchors = %v; want [foo.txt:1]", anchors)
	}
}

// newSessionWithDiff builds a session whose Scope has one tiny file
// with two `+` lines. Enough surface for read/skip round-trips.
func newSessionWithDiff(t *testing.T) *review.ReviewSession {
	t.Helper()
	patch := `diff --git a/foo.txt b/foo.txt
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/foo.txt
@@ -0,0 +1,2 @@
+line one
+line two`
	scope := review.Scope{
		Branch:  "feature",
		Base:    "main",
		TipSHA:  "abc1234567890",
		BaseSHA: "0000111122223333",
		Diff:    "main..feature",
		Title:   "feature",
		Files:   []string{"foo.txt"},
		RawDiff: patch,
		FilePatches: map[string]string{
			"foo.txt": patch,
		},
		CommitPatches: map[string]string{},
	}
	return review.New(scope, "tester@example.com", "")
}
