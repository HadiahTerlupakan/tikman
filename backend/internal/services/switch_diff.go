package services

import (
	"fmt"
	"strings"
)

const (
	diffContext = 3
	// diffMaxCells bounds the LCS table. A config of a few thousand lines with
	// a few edits stays far below it once the common head and tail are cut; a
	// rewrite past it is shown as one block removed and one added instead of
	// spending memory on a table nobody would read.
	diffMaxCells = 4_000_000
)

type diffOp struct {
	kind byte // ' ', '-' or '+'
	text string
}

// unifiedDiff renders the change from a to b in unified format, or "" when
// they are equal. It is computed here rather than by a library because the
// inputs are two masked configs and nothing more is needed.
func unifiedDiff(fromName, toName, a, b string) string {
	ops := diffLines(splitDiffLines(a), splitDiffLines(b))
	hunks := diffHunks(ops)
	if len(hunks) == 0 {
		return ""
	}
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ %s\n", fromName, toName)
	for _, h := range hunks {
		writeHunk(&out, ops, h)
	}
	return out.String()
}

func splitDiffLines(text string) []string {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func diffLines(a, b []string) []diffOp {
	head := 0
	for head < len(a) && head < len(b) && a[head] == b[head] {
		head++
	}
	tail := 0
	for tail < len(a)-head && tail < len(b)-head && a[len(a)-1-tail] == b[len(b)-1-tail] {
		tail++
	}
	ops := make([]diffOp, 0, len(a)+len(b))
	for _, line := range a[:head] {
		ops = append(ops, diffOp{' ', line})
	}
	ops = append(ops, diffMiddle(a[head:len(a)-tail], b[head:len(b)-tail])...)
	for _, line := range a[len(a)-tail:] {
		ops = append(ops, diffOp{' ', line})
	}
	return ops
}

func diffMiddle(a, b []string) []diffOp {
	if len(a)*len(b) > diffMaxCells {
		return replaceAll(a, b)
	}
	// lcs[i][j] is the longest common subsequence of a[i:] and b[j:].
	width := len(b) + 1
	lcs := make([]int32, (len(a)+1)*width)
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i*width+j] = lcs[(i+1)*width+j+1] + 1
			} else {
				lcs[i*width+j] = max(lcs[(i+1)*width+j], lcs[i*width+j+1])
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i]})
			i, j = i+1, j+1
		case lcs[(i+1)*width+j] >= lcs[i*width+j+1]:
			ops = append(ops, diffOp{'-', a[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j]})
			j++
		}
	}
	return append(ops, replaceAll(a[i:], b[j:])...)
}

func replaceAll(a, b []string) []diffOp {
	ops := make([]diffOp, 0, len(a)+len(b))
	for _, line := range a {
		ops = append(ops, diffOp{'-', line})
	}
	for _, line := range b {
		ops = append(ops, diffOp{'+', line})
	}
	return ops
}

// diffHunks returns [start, end) ranges of ops: each change with diffContext
// lines around it, merged when the ranges touch.
func diffHunks(ops []diffOp) [][2]int {
	var hunks [][2]int
	for i, op := range ops {
		if op.kind == ' ' {
			continue
		}
		start, end := max(i-diffContext, 0), min(i+diffContext+1, len(ops))
		if n := len(hunks); n > 0 && start <= hunks[n-1][1] {
			hunks[n-1][1] = end
		} else {
			hunks = append(hunks, [2]int{start, end})
		}
	}
	return hunks
}

func writeHunk(out *strings.Builder, ops []diffOp, h [2]int) {
	// Line numbers of the hunk's first line in each text.
	fromStart, toStart := 1, 1
	for _, op := range ops[:h[0]] {
		if op.kind != '+' {
			fromStart++
		}
		if op.kind != '-' {
			toStart++
		}
	}
	fromCount, toCount := 0, 0
	var body strings.Builder
	for _, op := range ops[h[0]:h[1]] {
		if op.kind != '+' {
			fromCount++
		}
		if op.kind != '-' {
			toCount++
		}
		body.WriteByte(op.kind)
		body.WriteString(op.text)
		body.WriteByte('\n')
	}
	fmt.Fprintf(out, "@@ -%d,%d +%d,%d @@\n%s", fromStart, fromCount, toStart, toCount, body.String())
}
