package chunker

import (
	"regexp"
	"strconv"
	"strings"
)

type headingStyle int

const (
	styleArabic headingStyle = iota + 1
	styleLetter
	styleRoman
	styleChapter
)

const (
	minHeadingsPerStyle = 2
	minHeadingGap       = 3
)

type line struct {
	Page  int
	Text  string
	Blank bool
}

type heading struct {
	Style   headingStyle
	Ordinal int
	Title   string
}

var (
	reArabicHeading  = regexp.MustCompile(`^\s*(\d+(\.\d+)*)[\.\)]?\s+(.{2,120})$`)
	reLetterHeading  = regexp.MustCompile(`^([A-Z])[\.\)]\s+(\S.{1,118})$`)
	reRomanHeading   = regexp.MustCompile(`^(X{0,3}(?:IX|IV|V?I{0,3}))[\.\)]\s+(\S.{1,118})$`)
	reChapterHeading = regexp.MustCompile(`(?i)^(BAB|BAGIAN|CHAPTER)\s+([IVXLC]{1,6}|\d{1,2})\b[\.\:\)]?\s*(.*)$`)
	reOrphanOrdinal  = regexp.MustCompile(`^(\d+(\.\d+)*|[A-Z]|X{0,3}(?:IX|IV|V?I{0,3}))[\.\)]$`)
	reHasLetter      = regexp.MustCompile(`\p{L}`)

	romanValues = map[byte]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100}

	monthWords = map[string]bool{
		"januari": true, "februari": true, "maret": true, "april": true, "mei": true, "juni": true,
		"juli": true, "agustus": true, "september": true, "oktober": true, "november": true, "desember": true,
		"january": true, "february": true, "march": true, "may": true, "june": true, "july": true,
		"august": true, "october": true, "december": true,
	}
)

func isLikelyHeading(text string) bool {
	if len(text) >= 120 {
		return false
	}
	m := reArabicHeading.FindStringSubmatch(text)
	if m == nil || len(strings.Split(m[1], ".")[0]) >= 4 {
		return false
	}
	return headingTitleOK(m[3])
}

func headingTitleOK(rest string) bool {
	rest = strings.TrimSpace(rest)
	if rest == "" || !reHasLetter.MatchString(rest) || strings.HasSuffix(rest, ".") {
		return false
	}
	return !monthWords[strings.ToLower(strings.Fields(rest)[0])]
}

func romanOrdinal(s string) int {
	total := 0
	for i := 0; i < len(s); i++ {
		v, ok := romanValues[s[i]]
		if !ok {
			return 0
		}
		if i+1 < len(s) {
			if next, ok := romanValues[s[i+1]]; ok && next > v {
				total -= v
				continue
			}
		}
		total += v
	}
	return total
}

func headingCandidates(text string) []heading {
	text = strings.TrimSpace(text)
	if text == "" || len(text) >= 120 {
		return nil
	}
	if m := reChapterHeading.FindStringSubmatch(text); m != nil {
		ord := romanOrdinal(strings.ToUpper(m[2]))
		if ord == 0 {
			ord, _ = strconv.Atoi(m[2])
		}
		return []heading{{Style: styleChapter, Ordinal: ord, Title: text}}
	}

	var out []heading
	if m := reArabicHeading.FindStringSubmatch(text); m != nil {
		first := strings.Split(m[1], ".")[0]
		if len(first) < 4 && headingTitleOK(m[3]) {
			ord, _ := strconv.Atoi(first)
			out = append(out, heading{Style: styleArabic, Ordinal: ord, Title: text})
		}
	}
	if m := reRomanHeading.FindStringSubmatch(text); m != nil && m[1] != "" && headingTitleOK(m[2]) {
		if ord := romanOrdinal(m[1]); ord > 0 {
			out = append(out, heading{Style: styleRoman, Ordinal: ord, Title: text})
		}
	}
	if m := reLetterHeading.FindStringSubmatch(text); m != nil && headingTitleOK(m[2]) {
		out = append(out, heading{Style: styleLetter, Ordinal: int(m[1][0]-'A') + 1, Title: text})
	}
	return out
}

func pickHeading(cands []heading, want, seen map[headingStyle]int) *heading {
	var picked *heading
	for i := range cands {
		c := &cands[i]
		switch c.Style {
		case styleChapter, styleArabic:
			if picked == nil || picked.Style == styleLetter || picked.Style == styleRoman {
				picked = c
			}
		case styleLetter, styleRoman:
			if c.Ordinal != want[c.Style] {
				continue
			}
			if picked == nil || ((picked.Style == styleLetter || picked.Style == styleRoman) && seen[c.Style] > seen[picked.Style]) {
				picked = c
			}
		}
	}
	return picked
}

func detectHeadings(lines []line) map[int]heading {
	accepted := map[int]heading{}
	want := map[headingStyle]int{styleLetter: 1, styleRoman: 1, styleArabic: 1}
	seen := map[headingStyle]int{}
	count := map[headingStyle]int{}
	lastIdx := map[headingStyle]int{}

	for i, ln := range lines {
		if ln.Blank {
			continue
		}
		h := pickHeading(headingCandidates(ln.Text), want, seen)
		if h == nil {
			continue
		}
		if h.Style == styleLetter || h.Style == styleRoman {
			want[h.Style]++
		}
		seen[h.Style]++
		if h.Style == styleArabic {
			if h.Ordinal != want[styleArabic] {
				continue
			}
			want[styleArabic]++
		}
		if count[h.Style] > 0 && i-lastIdx[h.Style] < minHeadingGap {
			continue
		}
		lastIdx[h.Style] = i
		count[h.Style]++
		accepted[i] = *h
	}

	for i, h := range accepted {
		if h.Style != styleChapter && count[h.Style] < minHeadingsPerStyle {
			delete(accepted, i)
		}
	}
	return accepted
}

func flatten(pages []Page) []line {
	var lines []line
	for _, pg := range pages {
		for _, raw := range strings.Split(pg.Text, "\n") {
			t := strings.TrimSpace(raw)
			lines = append(lines, line{Page: pg.Number, Text: t, Blank: t == ""})
		}
	}
	return joinOrphanOrdinals(lines)
}

func joinOrphanOrdinals(lines []line) []line {
	out := make([]line, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		if !ln.Blank && reOrphanOrdinal.MatchString(ln.Text) {
			if j := nextTextLine(lines, i+1); j >= 0 && len(lines[j].Text) <= 120 {
				ln.Text += " " + lines[j].Text
				out = append(out, ln)
				out = append(out, lines[i+1:j]...)
				i = j
				continue
			}
		}
		out = append(out, ln)
	}
	return out
}

func nextTextLine(lines []line, from int) int {
	for i := from; i < len(lines) && i < from+3; i++ {
		if !lines[i].Blank {
			return i
		}
	}
	return -1
}
