package chunker

import (
	"strings"
	"testing"
)

func TestChunkPagesSplitsOnNumberedHeadings(t *testing.T) {
	pages := []Page{{Number: 1, Text: "1. Purpose\nEnsure data can be restored.\n\n2. Scope\nAll production servers.\n\n3. Procedure\nRun a daily backup at 01:00."}}
	chunks := ChunkPages(pages)
	if len(chunks) != 3 {
		t.Fatalf("want 3 chunks, got %d", len(chunks))
	}
	if chunks[2].Section != "3. Procedure" || !strings.Contains(chunks[2].Content, "daily backup") {
		t.Fatalf("unexpected chunk: %+v", chunks[2])
	}
}

func TestDatesAreNotHeadings(t *testing.T) {
	pages := []Page{{Number: 1, Text: "1. Purpose\ntext\n\n12 January 2024\n\n2. Procedure\ntext"}}
	for _, c := range ChunkPages(pages) {
		if strings.HasPrefix(c.Section, "12") {
			t.Fatalf("date treated as heading: %q", c.Section)
		}
	}
}

func TestOrphanOrdinalIsJoined(t *testing.T) {
	pages := []Page{{Number: 1, Text: "1.\nPurpose\ntext\n\n2.\nProcedure\ntext"}}
	chunks := ChunkPages(pages)
	if len(chunks) != 2 || chunks[0].Section != "1. Purpose" {
		t.Fatalf("orphan ordinal not joined: %+v", chunks)
	}
}

func TestLongSectionIsSplitUnderLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString("1. Procedure\n")
	for i := 0; i < 60; i++ {
		b.WriteString(strings.Repeat("operational server step ", 10))
		b.WriteString("\n\n")
	}
	b.WriteString("2. Closing\ndone")
	chunks := ChunkPages([]Page{{Number: 1, Text: b.String()}})
	if len(chunks) < 3 {
		t.Fatalf("long section not split: %d chunks", len(chunks))
	}
	for _, c := range chunks[:len(chunks)-1] {
		if c.Section != "1. Procedure" || c.Tokens > MaxTokens {
			t.Fatalf("bad piece: section=%q tokens=%d", c.Section, c.Tokens)
		}
	}
}

func TestCleanPagesDropsRepeatedHeaderAndPageNumbers(t *testing.T) {
	pages := []Page{
		{Number: 1, Text: "Example Corp\n1. Purpose\ntext\nPage 1 of 3"},
		{Number: 2, Text: "Example Corp\n2. Procedure\ntext\nPage 2 of 3"},
		{Number: 3, Text: "Example Corp\n3. Closing\ntext\nPage 3 of 3"},
	}
	for _, p := range CleanPages(pages) {
		if strings.Contains(p.Text, "Example Corp") || strings.Contains(p.Text, "Page") {
			t.Fatalf("boilerplate kept: %q", p.Text)
		}
	}
}
