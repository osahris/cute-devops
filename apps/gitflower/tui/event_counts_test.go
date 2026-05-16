// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"strings"
	"testing"

	"gitflower/review"
)

// TestSidebarShowsEventCounters verifies the user's request: every
// Changes-tree row whose file has comments / questions / likes /
// dislikes appends an emoji counter so the reviewer can see at a
// glance which files have feedback on them.
func TestSidebarShowsEventCounters(t *testing.T) {
	sess := newSessionWithDiff(t)
	sess.AddComment(review.Comment{
		Anchor: review.Anchor("foo.txt:1"),
		Text:   "needs context",
		Kind:   review.KindComment,
	})
	sess.AddComment(review.Comment{
		Anchor: review.Anchor("foo.txt:2"),
		Text:   "why?",
		Kind:   review.KindQuestion,
	})
	sess.SetMarker(review.Anchor("foo.txt:1"), review.MarkerGood)
	sess.SetMarker(review.Anchor("foo.txt:2"), review.MarkerBad)

	m := newModel(sess, t.TempDir(), 1000.0)

	c := m.fileEventCounts("foo.txt")
	if c.comments != 1 || c.questions != 1 || c.likes != 1 || c.dislikes != 1 {
		t.Fatalf("counts wrong: %+v", c)
	}

	suf := c.suffix()
	for _, want := range []string{"💬 1", "❓ 1", "👍 1", "👎 1"} {
		if !strings.Contains(suf, want) {
			t.Errorf("suffix %q missing %q", suf, want)
		}
	}

	// The full Changes-tree row must include the suffix.
	m.changesRows = m.buildChangesRows()
	var fileRow *treeRow
	for i, row := range m.changesRows {
		if row.kind == tnFile && row.fileIdx >= 0 && m.files[row.fileIdx].Path == "foo.txt" {
			fileRow = &m.changesRows[i]
			break
		}
	}
	if fileRow == nil {
		t.Fatal("no Changes tree row for foo.txt")
	}
	rendered := m.renderTreeRow(*fileRow, sectionChanges)
	for _, want := range []string{"💬 1", "❓ 1", "👍 1", "👎 1"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered row %q missing %q", rendered, want)
		}
	}
}

// TestSidebarSuppressesZeroCounters confirms files with no events
// render as before — no trailing emoji noise.
func TestSidebarSuppressesZeroCounters(t *testing.T) {
	sess := newSessionWithDiff(t)
	m := newModel(sess, t.TempDir(), 1000.0)

	if got := m.fileEventCounts("foo.txt").suffix(); got != "" {
		t.Errorf("expected empty suffix for no events, got %q", got)
	}
}

// TestFolderCountersOnlyWhenCollapsed checks that an expanded folder
// row drops its rolled-up event suffix — the child file rows already
// carry their own counts, so summing them on the parent would
// double-render the same events.
func TestFolderCountersOnlyWhenCollapsed(t *testing.T) {
	// Build a session whose changed file lives inside a folder so the
	// Changes tree has an actual folder row to test against.
	patch := `diff --git a/pkg/foo.txt b/pkg/foo.txt
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/pkg/foo.txt
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
		Files:   []string{"pkg/foo.txt"},
		RawDiff: patch,
		FilePatches: map[string]string{
			"pkg/foo.txt": patch,
		},
		CommitPatches: map[string]string{},
	}
	sess := review.New(scope, "tester@example.com", "")
	sess.AddComment(review.Comment{
		Anchor: review.Anchor("pkg/foo.txt:1"),
		Text:   "needs context",
		Kind:   review.KindComment,
	})

	m := newModel(sess, t.TempDir(), 1000.0)
	m.changesRows = m.buildChangesRows()

	var dirRow *treeRow
	for i, row := range m.changesRows {
		if row.kind == tnDir && row.fullPath == "pkg" {
			dirRow = &m.changesRows[i]
			break
		}
	}
	if dirRow == nil {
		t.Fatal("no Changes tree row for pkg/ folder")
	}

	// Collapsed: roll-up shows. Explicit false overrides the
	// auto-expand-when-unread default.
	m.changesExpanded["pkg"] = false
	collapsed := m.renderTreeRow(*dirRow, sectionChanges)
	if !strings.Contains(collapsed, "💬 1") {
		t.Errorf("collapsed folder row missing rolled-up counter: %q", collapsed)
	}

	// Expanded: roll-up disappears — children render their own.
	m.changesExpanded["pkg"] = true
	expanded := m.renderTreeRow(*dirRow, sectionChanges)
	if strings.Contains(expanded, "💬") {
		t.Errorf("expanded folder row should drop roll-up, got: %q", expanded)
	}
}
