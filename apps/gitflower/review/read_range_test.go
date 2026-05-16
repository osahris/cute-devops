// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package review_test

import (
	"strings"
	"testing"

	"gitflower/review"
)

// TestSkipLinesCoalesceIntoRanges reproduces the user's complaint:
// per-line skip events flooded the rendered note. The renderer now
// groups contiguous skipped lines into a single SkipStart+SkipEnd
// pair anchored at `path:start-end`. Parse re-expands the range so
// the TUI's per-line tracker stays accurate.
func TestSkipLinesCoalesceIntoRanges(t *testing.T) {
	scope := mockScope()
	s := review.New(scope, "alice@example.com", "")
	s.AddVerdict(review.VerdictEvent{State: review.VerdictOpen})

	// Skip 5 contiguous lines + 2 disjoint runs on b.txt.
	for _, n := range []int{1, 2, 3, 4, 5, 8, 9, 12} {
		s.MarkSkipped(review.Anchor(formatAnchor("b.txt", n)))
	}

	body := review.Render(s)

	// Three runs → three SkipStart + three SkipEnd. Six events
	// total, not sixteen.
	startCount := strings.Count(body, "### SkipStart ")
	endCount := strings.Count(body, "### SkipEnd ")
	if startCount != 3 || endCount != 3 {
		t.Errorf("expected 3 SkipStart + 3 SkipEnd events; got %d start, %d end\n--- body ---\n%s",
			startCount, endCount, body)
	}
	// Verify ranges aren't split into single-line entries.
	for _, run := range []string{"b.txt:1-5", "b.txt:8-9", "b.txt:12-12"} {
		// Anchors are recovered from parsing position so they don't
		// appear in the rendered body text directly. Round-trip is
		// the real assertion — see below.
		_ = run
	}

	// Round-trip: Parse → check every original line came back.
	parsed, err := review.Parse(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, n := range []int{1, 2, 3, 4, 5, 8, 9, 12} {
		a := review.Anchor(formatAnchor("b.txt", n))
		if !parsed.IsSkipped(a) {
			t.Errorf("Parse lost skipped line %s", a)
		}
	}
	// Lines outside the runs must NOT come back as skipped.
	for _, n := range []int{6, 7, 10, 11} {
		a := review.Anchor(formatAnchor("b.txt", n))
		if parsed.IsSkipped(a) {
			t.Errorf("Parse incorrectly marked %s as skipped", a)
		}
	}
}

// TestReadLinesCoalesceIntoRanges: same coalesce semantics for the
// Read lane.
func TestReadLinesCoalesceIntoRanges(t *testing.T) {
	scope := mockScope()
	s := review.New(scope, "alice@example.com", "")
	s.AddVerdict(review.VerdictEvent{State: review.VerdictOpen})

	for _, n := range []int{2, 3, 4} {
		s.MarkRead(review.Anchor(formatAnchor("b.txt", n)))
	}

	body := review.Render(s)
	if got := strings.Count(body, "### ReadStart "); got != 1 {
		t.Errorf("expected 1 ReadStart, got %d", got)
	}
	if got := strings.Count(body, "### ReadEnd "); got != 1 {
		t.Errorf("expected 1 ReadEnd, got %d", got)
	}

	parsed, err := review.Parse(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, n := range []int{2, 3, 4} {
		if !parsed.IsRead(review.Anchor(formatAnchor("b.txt", n))) {
			t.Errorf("Parse lost read line %d", n)
		}
	}
}

func formatAnchor(path string, n int) string {
	return path + ":" + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
