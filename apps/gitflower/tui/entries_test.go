// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitflower/review"
)

// TestVerdictReplacesNotAppends verifies the one-verdict-per-user
// invariant: submitting a second verdict from the same author
// updates the existing entry rather than appending. Other users'
// verdicts stay untouched.
func TestVerdictReplacesNotAppends(t *testing.T) {
	sess := newSessionWithDiff(t)
	// Pre-existing verdict from a different reviewer — must not be
	// disturbed by the current user editing their own.
	sess.AddVerdict(review.VerdictEvent{
		State:   review.VerdictApproved,
		Author:  "other <other@example.com>",
		Summary: "lgtm",
	})

	sess.AddVerdict(review.VerdictEvent{
		State:   review.VerdictOpen,
		Summary: "first take",
	})
	sess.AddVerdict(review.VerdictEvent{
		State:   review.VerdictChanges,
		Summary: "second take",
	})

	vs := sess.Verdicts()
	if len(vs) != 2 {
		t.Fatalf("expected 2 verdicts (one per author), got %d", len(vs))
	}

	// Find the current reviewer's verdict — it should be the second
	// take, not the first.
	own := sess.VerdictIndexFor(sess.Reviewer)
	if own < 0 {
		t.Fatalf("VerdictIndexFor(reviewer) returned -1 after AddVerdict")
	}
	if vs[own].Summary != "second take" {
		t.Errorf("own verdict summary = %q; want %q",
			vs[own].Summary, "second take")
	}
	// The other reviewer's entry must still be present unchanged.
	otherFound := false
	for _, v := range vs {
		if v.Author == "other <other@example.com>" && v.State == review.VerdictApproved {
			otherFound = true
		}
	}
	if !otherFound {
		t.Errorf("other reviewer's verdict was clobbered: %v", vs)
	}
}

// TestVerdictsSidebarHasAddRow asserts the "+ Add verdict" sentinel
// appears when the current user has no verdict yet, and vanishes
// once they record one.
func TestVerdictsSidebarHasAddRow(t *testing.T) {
	sess := newSessionWithDiff(t)
	m := newModel(sess, t.TempDir(), 1000.0)

	items := m.sectionItems(sectionVerdicts)
	if len(items) == 0 || items[len(items)-1] != addVerdictRow {
		t.Errorf("expected last verdict item = %q; got %v", addVerdictRow, items)
	}

	// Record the user's own verdict.
	sess.AddVerdict(review.VerdictEvent{
		State:   review.VerdictApproved,
		Summary: "ok",
	})

	items = m.sectionItems(sectionVerdicts)
	for _, it := range items {
		if it == addVerdictRow {
			t.Errorf("'+ Add verdict' must disappear once the user has their own verdict; got %v",
				items)
		}
	}
}

// TestIssuesSidebarAlwaysHasAddRow asserts the "+ Add issue" sentinel
// is always the last entry — users can create arbitrarily many issues.
func TestIssuesSidebarAlwaysHasAddRow(t *testing.T) {
	sess := newSessionWithDiff(t)
	m := newModel(sess, t.TempDir(), 1000.0)

	items := m.sectionItems(sectionIssues)
	if len(items) != 1 || items[0] != addIssueRow {
		t.Fatalf("empty session should show only the add-row; got %v", items)
	}

	sess.AddIssue(review.Issue{Title: "ugly variable name", Body: "rename"})
	items = m.sectionItems(sectionIssues)
	if len(items) != 2 || items[1] != addIssueRow {
		t.Errorf("after adding one issue, items should be [title, add-row]; got %v", items)
	}
}

// TestVerdictKeyOpensEditorFromAnySection asserts pressing `v` in
// section mode opens the verdict editor regardless of which section
// the cursor is on — it's the "global verdict shortcut".
func TestVerdictKeyOpensEditorFromAnySection(t *testing.T) {
	sess := newSessionWithDiff(t)
	m := newModel(sess, t.TempDir(), 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	m.sect = sectionChanges
	m = key(t, m, 'v', "v")
	if m.edit != editSummary {
		t.Errorf("after `v` from sectionChanges: edit=%v; want editSummary", m.edit)
	}
}

// TestIssueKeyOpensEditorFromAnySection: same idea for `i`. It must
// open the new-issue editor with the title field focused, regardless
// of which section the cursor is on.
func TestIssueKeyOpensEditorFromAnySection(t *testing.T) {
	sess := newSessionWithDiff(t)
	m := newModel(sess, t.TempDir(), 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	m.sect = sectionChanges
	m = key(t, m, 'i', "i")
	if m.edit != editIssue {
		t.Fatalf("after `i` from sectionChanges: edit=%v; want editIssue", m.edit)
	}
	if !m.title.Focused() {
		t.Errorf("issue editor should focus title input first")
	}
	if m.editIssIdx != -1 {
		t.Errorf("editIssIdx = %d; want -1 (new issue)", m.editIssIdx)
	}
}

// TestDeleteIssueViaSidebar exercises the `d` key on an existing
// issue row: the issue must vanish from the session.
func TestDeleteIssueViaSidebar(t *testing.T) {
	sess := newSessionWithDiff(t)
	sess.AddIssue(review.Issue{Title: "keep me"})
	sess.AddIssue(review.Issue{Title: "delete me", Body: "go away"})

	m := newModel(sess, t.TempDir(), 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m.sect = sectionIssues
	m.sectIdx[sectionIssues] = 1 // cursor on "delete me"

	m = key(t, m, 'd', "d")

	if len(sess.Issues()) != 1 {
		t.Fatalf("expected 1 issue left, got %d", len(sess.Issues()))
	}
	if sess.Issues()[0].Title != "keep me" {
		t.Errorf("wrong issue survived: %q", sess.Issues()[0].Title)
	}
}

// TestDeleteOwnVerdictViaSidebar: `d` on the user's own verdict row
// removes it; the canonical `Verdict`/`Summary` falls back to the
// previous state (or VerdictOpen when none).
func TestDeleteOwnVerdictViaSidebar(t *testing.T) {
	sess := newSessionWithDiff(t)
	sess.AddVerdict(review.VerdictEvent{State: review.VerdictApproved, Summary: "ok"})

	m := newModel(sess, t.TempDir(), 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m.sect = sectionVerdicts
	m.sectIdx[sectionVerdicts] = 0 // own verdict (no add-row when one exists)

	m = key(t, m, 'd', "d")

	if len(sess.Verdicts()) != 0 {
		t.Errorf("expected 0 verdicts after delete, got %d", len(sess.Verdicts()))
	}
	if sess.Verdict != review.VerdictOpen {
		t.Errorf("canonical Verdict after delete = %q; want %q",
			sess.Verdict, review.VerdictOpen)
	}
}

// TestVerdictEditorAcceptsInput is the regression test for the
// "verdict hangs" bug: pressing `v` opened the editor but the
// textarea's focus tea.Cmd was being dropped, so subsequent
// keypresses had no effect. The fix queues the focus cmd via
// queuedCmds. We verify by pressing `v`, typing a body, then Enter
// and checking the verdict actually got recorded.
func TestVerdictEditorAcceptsInput(t *testing.T) {
	sess := newSessionWithDiff(t)
	m := newModel(sess, t.TempDir(), 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	m.sect = sectionChanges
	m = key(t, m, 'v', "v")
	if m.edit != editSummary {
		t.Fatalf("after `v`: edit=%v; want editSummary", m.edit)
	}
	for _, r := range "looks good" {
		m = key(t, m, r, string(r))
	}
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.edit != editNone {
		t.Errorf("Enter should have submitted; edit still = %v", m.edit)
	}
	vs := sess.Verdicts()
	if len(vs) != 1 {
		t.Fatalf("expected 1 verdict, got %d", len(vs))
	}
	if vs[0].Summary != "looks good" {
		t.Errorf("verdict summary = %q; want %q", vs[0].Summary, "looks good")
	}
}

// TestIssueEditorAcceptsInput is the matching regression test for `i`.
func TestIssueEditorAcceptsInput(t *testing.T) {
	sess := newSessionWithDiff(t)
	m := newModel(sess, t.TempDir(), 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	m.sect = sectionChanges
	m = key(t, m, 'i', "i")
	if m.edit != editIssue {
		t.Fatalf("after `i`: edit=%v; want editIssue", m.edit)
	}
	for _, r := range "weird API" {
		m = key(t, m, r, string(r))
	}
	// Enter on title advances to body.
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	for _, r := range "rename" {
		m = key(t, m, r, string(r))
	}
	// Enter on body submits.
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.edit != editNone {
		t.Errorf("body Enter should have submitted; edit still = %v", m.edit)
	}
	issues := sess.Issues()
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Title != "weird API" || issues[0].Body != "rename" {
		t.Errorf("issue = %+v; want {weird API, rename}", issues[0])
	}
}

// TestDeleteHunkAnchoredQuestionFromLine reproduces the user's
// complaint: a hunk-anchored question (`path:start,count`) could not
// be deleted from any line cursor because CommentIndexAt only did
// exact-anchor matching. After the fix, putting the line cursor
// anywhere inside the surrounding hunk lets `d` find and remove the
// question.
func TestDeleteHunkAnchoredQuestionFromLine(t *testing.T) {
	sess := newSessionWithDiff(t)
	// foo.txt has one hunk @@ +1,2 — anchor the question at the
	// hunk, not at any single line.
	sess.AddComment(review.Comment{
		Anchor: review.Anchor("foo.txt:1,2"),
		Text:   "is this design intentional?",
		Kind:   review.KindQuestion,
	})
	if len(sess.Comments()) != 1 {
		t.Fatalf("setup: expected 1 question, got %d", len(sess.Comments()))
	}

	m := newModel(sess, t.TempDir(), 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	// Drill into Changes, cursor lands on line 1 of foo.txt — which
	// is INSIDE the hunk but not the anchor itself.
	m = key(t, m, ' ', " ")
	if m.mode != modeDiff {
		t.Fatalf("expected modeDiff after Space, got %v", m.mode)
	}

	m = key(t, m, 'd', "d")

	if got := len(sess.Comments()); got != 0 {
		t.Errorf("expected 0 comments after delete, got %d", got)
	}
}

// TestCycleCommentCursor: two events on the same hunk — `]` selects
// the first, again selects the second, e on the selected one acts.
func TestCycleCommentCursor(t *testing.T) {
	sess := newSessionWithDiff(t)
	sess.AddComment(review.Comment{Anchor: review.Anchor("foo.txt:1"), Text: "first"})
	sess.AddComment(review.Comment{Anchor: review.Anchor("foo.txt:1,2"), Text: "second", Kind: review.KindQuestion})

	m := newModel(sess, t.TempDir(), 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = key(t, m, ' ', " ")

	if m.commentCursor != -1 {
		t.Fatalf("commentCursor should start at -1, got %d", m.commentCursor)
	}
	m = key(t, m, ']', "]")
	if m.commentCursor != 0 {
		t.Errorf("first ] selects idx 0, got %d", m.commentCursor)
	}
	m = key(t, m, ']', "]")
	if m.commentCursor != 1 {
		t.Errorf("second ] selects idx 1, got %d", m.commentCursor)
	}
	m = key(t, m, 'd', "d")
	if len(sess.Comments()) != 1 || sess.Comments()[0].Text != "first" {
		t.Errorf("d on selected should drop only the 2nd comment; got %v", sess.Comments())
	}
}

// TestEditorEnterSubmits checks the editor's "Enter saves, Alt+Enter
// newlines" contract: typing a body then pressing plain Enter must
// land the entry in the session without a trailing newline.
func TestEditorEnterSubmits(t *testing.T) {
	sess := newSessionWithDiff(t)
	m := newModel(sess, t.TempDir(), 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Drill into the diff so we can leave a comment.
	m = key(t, m, ' ', " ")
	m = key(t, m, 'c', "c")
	if m.edit != editComment {
		t.Fatalf("expected editComment, got %v", m.edit)
	}
	for _, r := range "single line" {
		m = key(t, m, r, string(r))
	}
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.edit != editNone {
		t.Errorf("plain Enter should submit; edit still = %v", m.edit)
	}
	if len(sess.Comments()) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(sess.Comments()))
	}
	if got := sess.Comments()[0].Text; got != "single line" || strings.Contains(got, "\n") {
		t.Errorf("comment text = %q; want %q without newline",
			got, "single line")
	}
}

// TestEditorAltEnterInsertsNewline checks Alt+Enter's role: it must
// add a literal "\n" to the body and keep the editor open.
func TestEditorAltEnterInsertsNewline(t *testing.T) {
	sess := newSessionWithDiff(t)
	m := newModel(sess, t.TempDir(), 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	m = key(t, m, ' ', " ")
	m = key(t, m, 'c', "c")

	for _, r := range "line one" {
		m = key(t, m, r, string(r))
	}
	// Alt+Enter inserts a newline rather than submitting.
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	if m.edit != editComment {
		t.Fatalf("Alt+Enter must not submit; edit = %v", m.edit)
	}
	for _, r := range "line two" {
		m = key(t, m, r, string(r))
	}
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if len(sess.Comments()) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(sess.Comments()))
	}
	got := sess.Comments()[0].Text
	if got != "line one\nline two" {
		t.Errorf("comment text = %q; want %q", got, "line one\nline two")
	}
}
