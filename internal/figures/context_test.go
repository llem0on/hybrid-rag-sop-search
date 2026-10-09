package figures

import "testing"

func TestContextFindsHeadingAndCaption(t *testing.T) {
	lines := map[int][]Line{
		4: {{Top: 300, Text: "4.1 Risk Assessment"}},
		5: {
			{Top: 200, Text: "4.2. Likelihood and Impact"},
			{Top: 420, Text: "Impact Matrix"},
			{Top: 800, Text: "After the matrix is filled, the owner signs it."},
		},
	}
	heading, caption := Context(lines, 5, 450, 300)
	if heading != "4.2. Likelihood and Impact" || caption != "Impact Matrix" {
		t.Fatalf("got heading=%q caption=%q", heading, caption)
	}
	if got := SectionTitle(5, heading, caption); got != "Figure on page 5 · 4.2. Likelihood and Impact - Impact Matrix" {
		t.Fatalf("unexpected title %q", got)
	}
}

func TestContextSearchesEarlierPagesAndCaptionBelow(t *testing.T) {
	lines := map[int][]Line{
		2: {{Top: 300, Text: "3.1 Organization Structure"}},
		3: {{Top: 520, Text: "Figure 1. Team chart"}},
	}
	heading, caption := Context(lines, 3, 200, 300)
	if heading != "3.1 Organization Structure" || caption != "Figure 1. Team chart" {
		t.Fatalf("got heading=%q caption=%q", heading, caption)
	}
}

func TestGroupLinesSkipsHeaderAndFooterBands(t *testing.T) {
	cells := []textCell{
		{top: 50, left: 10, text: "Company header"},
		{top: 500, left: 200, text: "World"},
		{top: 501, left: 10, text: "Hello"},
		{top: 980, left: 10, text: "Page 1"},
	}
	got := groupLines(cells, 1000)
	if len(got) != 1 || got[0].Text != "Hello World" {
		t.Fatalf("unexpected lines %+v", got)
	}
}
