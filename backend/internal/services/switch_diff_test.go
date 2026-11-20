package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const diffBefore = `sysname SW-TEST
vlan batch 10 20
interface 10GE1/0/1
 port default vlan 10
interface 10GE1/0/2
 port default vlan 20
return
`

const diffAfter = `sysname SW-TEST
vlan batch 10 20 30
interface 10GE1/0/1
 port default vlan 10
interface 10GE1/0/2
 port default vlan 30
return
`

func TestUnifiedDiffShowsOnlyTheChangedLinesWithContext(t *testing.T) {
	got := unifiedDiff("2026-11-01", "2026-11-02", diffBefore, diffAfter)

	want := strings.Join([]string{
		"--- 2026-11-01",
		"+++ 2026-11-02",
		"@@ -1,7 +1,7 @@",
		" sysname SW-TEST",
		"-vlan batch 10 20",
		"+vlan batch 10 20 30",
		" interface 10GE1/0/1",
		"  port default vlan 10",
		" interface 10GE1/0/2",
		"- port default vlan 20",
		"+ port default vlan 30",
		" return",
		"",
	}, "\n")
	assert.Equal(t, want, got)
}

func TestUnifiedDiffOfEqualTextsHasNoHunks(t *testing.T) {
	assert.Empty(t, unifiedDiff("a", "b", diffBefore, diffBefore))
}

func TestUnifiedDiffSplitsDistantChangesIntoHunks(t *testing.T) {
	var before, after []string
	for i := 0; i < 20; i++ {
		before = append(before, "line"+string(rune('a'+i)))
		after = append(after, "line"+string(rune('a'+i)))
	}
	after[1], after[18] = "changed-1", "changed-18"

	got := unifiedDiff("a", "b", strings.Join(before, "\n"), strings.Join(after, "\n"))

	assert.Equal(t, 2, strings.Count(got, "@@ -"))
	assert.Contains(t, got, "-lineb\n+changed-1")
	assert.Contains(t, got, "-lines\n+changed-18")
}

func TestUnifiedDiffOfAHugeChangeStillAnswers(t *testing.T) {
	var a, b []string
	for i := 0; i < 3000; i++ {
		a = append(a, "a")
		b = append(b, "b")
	}
	got := unifiedDiff("a", "b", strings.Join(a, "\n"), strings.Join(b, "\n"))
	assert.Contains(t, got, "-a\n")
	assert.Contains(t, got, "+b\n")
}
