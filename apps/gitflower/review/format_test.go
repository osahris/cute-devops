// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package review_test

import (
	"fmt"
	"strings"
	"testing"

	"gitflower/review"
)

func mockScope() review.Scope {
	return review.Scope{
		Branch:  "mr/feat-b",
		Base:    "main",
		TipSHA:  "abc1234567890",
		BaseSHA: "0000111122223333",
		Diff:    "main..mr/feat-b",
		Title:   "Add feature B",
		Commits: []review.Commit{
			{SHA: "abc1234567890", Short: "abc1234567890", Subject: "Add feature B"},
		},
		Files: []string{"b.txt", "b.test"},
		FilePatches: map[string]string{
			"b.txt": `diff --git a/b.txt b/b.txt
new file mode 100644
index 0000000..0123456
--- /dev/null
+++ b/b.txt
@@ -0,0 +1,3 @@
+feature B
+initial implementation
+line 3`,
			"b.test": `diff --git a/b.test b/b.test
new file mode 100644
index 0000000..7654321
--- /dev/null
+++ b/b.test
@@ -0,0 +1 @@
+test for B`,
		},
		CommitPatches: map[string]string{
			"abc1234567890": `From abc1234567890 Mon Sep 17 00:00:00 2001
From: Author <author@demo>
Subject: [PATCH] Add feature B

---
 b.txt | 3 +++
 1 file changed, 3 insertions(+)

diff --git a/b.txt b/b.txt
new file mode 100644
--- /dev/null
+++ b/b.txt
@@ -0,0 +1,3 @@
+feature B
+initial implementation
+line 3
`,
		},
	}
}

func TestRenderProducesExpectedSections(t *testing.T) {
	s := review.New(mockScope(), "alice@example.com", "/tmp/test.review")
	s.AddVerdict(review.VerdictEvent{State: review.VerdictChanges, Summary: "Needs a test."})
	s.AddIssue(review.Issue{Title: "follow project style"})

	out := review.Render(s)

	for _, want := range []string{
		"# Review\n",
		"## Sources\n",
		"## Verdicts\n",
		"### Verdict: requested-changes (From: alice <alice@example.com>",
		"# General Issues\n",
		"## Issue 1: follow project style",
		"# Changes\n",
		"## Changes in `b.txt`",
		"> diff --git a/b.txt b/b.txt",
		"> +feature B",
		"## Changes in `b.test`",
		"# Commits\n",
		"## Commit abc1234567890: Add feature B",
		"> From abc1234567890",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- BEGIN OUTPUT ---\n%s\n--- END OUTPUT ---", want, out)
			return
		}
	}
}

func TestRoundTripEventsAndIssues(t *testing.T) {
	s := review.New(mockScope(), "alice@example.com", "/tmp/test.review")
	s.AddVerdict(review.VerdictEvent{State: review.VerdictChanges, Summary: "Needs a test."})

	// Hunk-anchored Like + Read
	s.SetMarker(review.HunkAnchor("b.txt", 1, 3), review.MarkerGood)
	s.MarkRead(review.HunkAnchor("b.txt", 1, 3))

	// Line-anchored comment
	s.AddComment(review.Comment{
		Anchor: review.Anchor("b.txt:2"),
		Text:   "Naming nit.",
		Kind:   review.KindComment,
	})

	// Hunk-anchored question
	s.AddComment(review.Comment{
		Anchor: review.Anchor("b.test:1,1"),
		Text:   "Is one line enough?",
		Kind:   review.KindQuestion,
	})

	// Hunk-anchored Dislike
	s.SetMarker(review.HunkAnchor("b.test", 1, 1), review.MarkerBad)

	// General issue
	s.AddIssue(review.Issue{Title: "follow project style", Body: "Several names are unclear."})

	out := review.Render(s)

	parsed, err := review.Parse(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed.Reviewer != "alice@example.com" {
		t.Errorf("reviewer: got %q want alice@example.com", parsed.Reviewer)
	}
	if parsed.Verdict != review.VerdictChanges {
		t.Errorf("verdict: got %q", parsed.Verdict)
	}
	// Markers stay in the same hunk but may drift from hunk anchor
	// (`path:start,count`) to the hunk's last `+` line (`path:N`)
	// because Render emits them after the last new-side line and
	// Parse derives the anchor from the preceding `> +N`. Both
	// shapes are accepted; what matters is the marker is still set
	// on a position inside the original hunk.
	if got := markerInHunk(parsed, "b.txt", 1, 3, review.MarkerGood); !got {
		t.Errorf("b.txt good marker lost; markers: %v", parsed.MarkerAnchors())
	}
	if got := markerInHunk(parsed, "b.test", 1, 1, review.MarkerBad); !got {
		t.Errorf("b.test bad marker lost; markers: %v", parsed.MarkerAnchors())
	}
	if !parsed.IsRead(review.HunkAnchor("b.txt", 1, 3)) {
		t.Errorf("b.txt:1,3 read marker lost")
	}
	if len(parsed.Comments()) < 2 {
		t.Errorf("comments: got %d want at least 2", len(parsed.Comments()))
	}
	if len(parsed.Issues()) != 1 {
		t.Errorf("issues: got %d want 1", len(parsed.Issues()))
	} else if parsed.Issues()[0].Title != "follow project style" {
		t.Errorf("issue title: got %q", parsed.Issues()[0].Title)
	}
}

// markerInHunk returns true if `s` carries `want` somewhere inside
// the hunk at (newStart, newLines) of path — either at the exact
// hunk anchor `path:start,count` or at any `+` line inside the hunk
// (path:N where newStart <= N <= newStart+newLines-1).
func markerInHunk(s *review.ReviewSession, path string, newStart, newLines int, want review.Marker) bool {
	if s.Marker(review.HunkAnchor(path, newStart, newLines)) == want {
		return true
	}
	for n := newStart; n < newStart+newLines; n++ {
		if s.Marker(review.Anchor(fmt.Sprintf("%s:%d", path, n))) == want {
			return true
		}
	}
	return false
}

func TestPathEncodingRoundTrip(t *testing.T) {
	// For each input, encoded form must round-trip back to the original.
	for _, raw := range []string{
		"simple.txt",
		"path/to/file.go",
		"weird name.txt",
		"héllo.txt",
		"back`tick.txt",
		"back\\slash.txt",
	} {
		enc := review.EncodePath(raw)
		got := review.DecodePath(enc)
		if got != raw {
			t.Errorf("round-trip %q: encoded=%q decoded=%q", raw, enc, got)
		}
	}
}
