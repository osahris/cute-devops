// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package review_test

import (
	"path/filepath"
	"testing"

	"gitflower/review"
)

// TestRoundTrip checks that a Session with read markers, good/bad markers,
// comments, questions, and a verdict can be Saved and Loaded losslessly.
func TestRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	scope := review.Scope{
		Branch:  "mr/feat-b",
		Base:    "main",
		TipSHA:  "abc1234567890",
		Diff:    "main..mr/feat-b",
		Title:   "Add feature B",
		Commits: []review.Commit{{SHA: "abc123", Short: "abc123", Subject: "work"}},
		Files:   []string{"b.txt", "b.test"},
		RawDiff: "",
	}
	path := filepath.Join(tmp, "x.alice.review.md")
	s := review.New(scope, "alice@example.com", path)
	s.MarkRead(review.HunkAnchor("b.txt", 1, 3))
	s.SetMarker(review.HunkAnchor("b.txt", 1, 3), review.MarkerGood)
	s.SetMarker(review.HunkAnchor("b.test", 1, 1), review.MarkerBad)
	s.AddComment(review.Comment{
		Anchor: review.Anchor("b.txt:1-3"),
		Author: "alice@example.com",
		Date:   "2026-05-15T18:00:00Z",
		Text:   "needs a test",
		Kind:   review.KindComment,
	})
	s.AddComment(review.Comment{
		Anchor: review.Anchor("b.test:1"),
		Author: "alice@example.com",
		Date:   "2026-05-15T18:01:00Z",
		Text:   "is this case intentional?",
		Kind:   review.KindQuestion,
	})
	s.SetVerdict(review.VerdictChanges)
	s.SetSummary("Needs a test before merge.")
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := review.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Reviewer != "alice@example.com" {
		t.Errorf("reviewer: got %q", loaded.Reviewer)
	}
	if loaded.Verdict != review.VerdictChanges {
		t.Errorf("verdict: got %q", loaded.Verdict)
	}
	if !loaded.IsRead(review.HunkAnchor("b.txt", 1, 3)) {
		t.Errorf("read marker for b.txt:1,3 lost")
	}
	if loaded.Marker(review.HunkAnchor("b.txt", 1, 3)) != review.MarkerGood {
		t.Errorf("good marker for b.txt:1,3 lost")
	}
	if loaded.Marker(review.HunkAnchor("b.test", 1, 1)) != review.MarkerBad {
		t.Errorf("bad marker for b.test:1,1 lost")
	}
	cs := loaded.Comments()
	if len(cs) != 2 {
		t.Fatalf("comments: got %d, want 2", len(cs))
	}
	var sawComment, sawQuestion bool
	for _, c := range cs {
		if c.Kind == review.KindQuestion {
			sawQuestion = true
		} else {
			sawComment = true
		}
	}
	if !sawComment || !sawQuestion {
		t.Errorf("kinds lost: comment=%v question=%v", sawComment, sawQuestion)
	}
	if loaded.Summary != "Needs a test before merge." {
		t.Errorf("summary: got %q", loaded.Summary)
	}
}
