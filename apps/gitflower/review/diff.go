// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package review

import (
	"bufio"
	"strconv"
	"strings"
)

// File is one file's worth of parsed diff.
type File struct {
	Path  string // new path (or old, if deletion)
	Old   string // old path; "" for new files
	Hunks []Hunk
}

// Hunk is one @@-delimited region.
type Hunk struct {
	Header   string // raw "@@ -a,b +c,d @@" line
	OldStart int    // a
	OldLines int    // b
	NewStart int    // c
	NewLines int    // d
	Lines    []Line // including context, additions, deletions
}

// Line is one line in a hunk.
type Line struct {
	Kind LineKind
	Text string // raw text without the leading +/-/space
}

// LineKind is the role of a line in a hunk.
type LineKind int

const (
	LineContext LineKind = iota
	LineAdd
	LineDelete
)

// ParseDiff turns a unified diff (the output of `git diff`) into a slice of File.
// Tolerant of binary diffs, mode changes, and rename headers — they appear as
// Files with no Hunks.
func ParseDiff(diff string) []File {
	var files []File
	var cur *File
	var curHunk *Hunk

	sc := bufio.NewScanner(strings.NewReader(diff))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			if cur != nil {
				if curHunk != nil {
					cur.Hunks = append(cur.Hunks, *curHunk)
					curHunk = nil
				}
				files = append(files, *cur)
			}
			cur = &File{Path: extractDiffGitPath(line)}
		case strings.HasPrefix(line, "--- "):
			if cur != nil {
				cur.Old = stripABPrefix(strings.TrimPrefix(line, "--- "))
			}
		case strings.HasPrefix(line, "+++ "):
			if cur != nil {
				p := stripABPrefix(strings.TrimPrefix(line, "+++ "))
				if p != "" && p != "/dev/null" {
					cur.Path = p
				}
			}
		case strings.HasPrefix(line, "@@"):
			if cur == nil {
				continue
			}
			if curHunk != nil {
				cur.Hunks = append(cur.Hunks, *curHunk)
			}
			curHunk = parseHunkHeader(line)
		case curHunk != nil && len(line) > 0:
			switch line[0] {
			case '+':
				curHunk.Lines = append(curHunk.Lines, Line{Kind: LineAdd, Text: line[1:]})
			case '-':
				curHunk.Lines = append(curHunk.Lines, Line{Kind: LineDelete, Text: line[1:]})
			case ' ':
				curHunk.Lines = append(curHunk.Lines, Line{Kind: LineContext, Text: line[1:]})
			case '\\':
				// "\ No newline at end of file" — skip
			}
		}
	}
	if cur != nil {
		if curHunk != nil {
			cur.Hunks = append(cur.Hunks, *curHunk)
		}
		files = append(files, *cur)
	}
	return files
}

func extractDiffGitPath(line string) string {
	// `diff --git a/path/to/foo b/path/to/foo`
	rest := strings.TrimPrefix(line, "diff --git ")
	fields := strings.Fields(rest)
	if len(fields) < 2 {
		return ""
	}
	return stripABPrefix(fields[1])
}

func stripABPrefix(p string) string {
	p = strings.TrimSpace(p)
	if strings.HasPrefix(p, "a/") || strings.HasPrefix(p, "b/") {
		return p[2:]
	}
	return p
}

func parseHunkHeader(line string) *Hunk {
	// "@@ -a,b +c,d @@ optional context"
	h := &Hunk{Header: line}
	rest := strings.TrimPrefix(line, "@@")
	end := strings.Index(rest, "@@")
	if end < 0 {
		return h
	}
	spec := strings.TrimSpace(rest[:end])
	parts := strings.Fields(spec)
	for _, p := range parts {
		if strings.HasPrefix(p, "-") {
			h.OldStart, h.OldLines = parseRange(p[1:])
		} else if strings.HasPrefix(p, "+") {
			h.NewStart, h.NewLines = parseRange(p[1:])
		}
	}
	return h
}

func parseRange(s string) (start, count int) {
	count = 1
	if i := strings.IndexByte(s, ','); i >= 0 {
		start, _ = strconv.Atoi(s[:i])
		count, _ = strconv.Atoi(s[i+1:])
	} else {
		start, _ = strconv.Atoi(s)
	}
	return
}

// NewLineRange returns the inclusive new-side line range covered by the hunk
// (e.g. "10-15"). If the hunk has zero new lines, returns the deletion point
// as a single line.
func (h Hunk) NewLineRange() string {
	if h.NewLines == 0 {
		return strconv.Itoa(h.NewStart)
	}
	if h.NewLines == 1 {
		return strconv.Itoa(h.NewStart)
	}
	return strconv.Itoa(h.NewStart) + "-" + strconv.Itoa(h.NewStart+h.NewLines-1)
}
