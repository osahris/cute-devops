// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package review_test

import (
	"testing"

	"gitflower/review"
)

// TestMarkerLineAnchorRoundtrips reproduces the user's report:
// "it read the comment but not the like." A Like placed on a single
// `+` line gets anchored as `path:N` (line anchor). After Render →
// Parse, the marker's anchor must still resolve to the same position
// so the TUI can display it.
//
// Current bug: Parse forces Like/Dislike/Read/Skip anchors to the
// surrounding hunk anchor (`path:start,count`), so a line-anchored
// Like comes back as a hunk-anchored marker on a different key.
// SetMarker stored `path:2`, after roundtrip it's `path:1,3`.
func TestMarkerLineAnchorRoundtrips(t *testing.T) {
	scope := mockScope()
	s := review.New(scope, "alice@example.com", "")
	s.AddVerdict(review.VerdictEvent{State: review.VerdictOpen})

	// Both a line-anchored Like and a line-anchored Comment on the
	// same file. Same shape; either both survive or both don't.
	lineLike := review.Anchor("b.txt:2")
	lineComment := review.Anchor("b.txt:2")
	s.SetMarker(lineLike, review.MarkerGood)
	s.AddComment(review.Comment{Anchor: lineComment, Text: "needs context"})

	body := review.Render(s)

	parsed, err := review.Parse(body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// The comment must come back at its original line anchor.
	if got := parsed.Comments(); len(got) != 1 || got[0].Anchor != lineComment {
		t.Errorf("comment anchor lost: got %+v, want anchor=%q",
			got, lineComment)
	}

	// The marker must come back at the same line anchor.
	if parsed.Marker(lineLike) != review.MarkerGood {
		t.Errorf("Like at %q not preserved through Render→Parse; markers map = %v\n--- body ---\n%s",
			lineLike, parsed.MarkerAnchors(), body)
	}
}
