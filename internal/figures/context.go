package figures

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	captionAbovePx = 60
	captionBelowPx = 30
	labelMaxRunes  = 80
	headerBand     = 17
	footerBand     = 90
)

var reNumberedHeading = regexp.MustCompile(`^(\d+(\.\d+)+\.?|\d+\.)\s+[A-Za-z]{3,}`)

type Line struct {
	Top  int
	Text string
}

type textCell struct {
	top, left int
	text      string
}

func groupLines(cells []textCell, pageHeight int) []Line {
	rows := map[int][]textCell{}
	for _, c := range cells {
		rows[c.top/6] = append(rows[c.top/6], c)
	}
	keys := make([]int, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	var out []Line
	for _, k := range keys {
		row := rows[k]
		sort.Slice(row, func(i, j int) bool { return row[i].left < row[j].left })
		top := row[0].top
		if pageHeight > 0 && (top*100 < pageHeight*headerBand || top*100 > pageHeight*footerBand) {
			continue
		}
		parts := make([]string, len(row))
		for i, c := range row {
			parts[i] = c.text
		}
		out = append(out, Line{Top: top, Text: strings.Join(strings.Fields(strings.Join(parts, " ")), " ")})
	}
	return out
}

func Context(lines map[int][]Line, page, top, height int) (heading, caption string) {
	for p := page; p >= 1 && heading == ""; p-- {
		for _, l := range lines[p] {
			if p == page && l.Top >= top {
				break
			}
			if reNumberedHeading.MatchString(l.Text) {
				heading = l.Text
			}
		}
	}

	bestAbove := captionAbovePx + 1
	for _, l := range lines[page] {
		if d := top - l.Top; d >= 0 && d < bestAbove {
			bestAbove, caption = d, l.Text
		}
	}
	if caption == "" {
		bestBelow := captionBelowPx + 1
		for _, l := range lines[page] {
			if d := l.Top - (top + height); d >= 0 && d < bestBelow {
				bestBelow, caption = d, l.Text
			}
		}
	}
	if caption == heading {
		caption = ""
	}
	return truncate(heading), truncate(caption)
}

func SectionTitle(page int, heading, caption string) string {
	label := heading
	if caption != "" {
		if label != "" {
			label += " - "
		}
		label += caption
	}
	title := "Figure on page " + strconv.Itoa(page)
	if label != "" {
		title += " · " + label
	}
	return title
}

func truncate(s string) string {
	r := []rune(s)
	if len(r) > labelMaxRunes {
		return strings.TrimSpace(string(r[:labelMaxRunes]))
	}
	return s
}
