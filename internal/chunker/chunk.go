package chunker

import (
	"regexp"
	"strings"
)

const (
	CharsPerToken = 4
	TargetTokens  = 500
	MaxTokens     = 700
	OverlapWords  = 80
)

type Chunk struct {
	Section   string
	PageStart int
	PageEnd   int
	Content   string
	Tokens    int
}

type section struct {
	title     string
	pageStart int
	pageEnd   int
	body      strings.Builder
}

var reParagraphBreak = regexp.MustCompile(`\n{2,}`)

func ChunkPages(pages []Page) []Chunk {
	var chunks []Chunk
	for _, sec := range splitSections(pages) {
		body := strings.TrimSpace(sec.body.String())
		if body == "" && sec.title == "" {
			continue
		}
		full := withTitle(sec.title, body)
		if EstimateTokens(full) <= MaxTokens {
			chunks = append(chunks, newChunk(sec, full))
			continue
		}
		for _, piece := range splitLongSection(body) {
			chunks = append(chunks, newChunk(sec, withTitle(sec.title, piece)))
		}
	}
	return chunks
}

func splitSections(pages []Page) []*section {
	lines := flatten(pages)
	heads := detectHeadings(lines)

	var sections []*section
	var cur *section
	for i, ln := range lines {
		if ln.Blank {
			if cur != nil {
				cur.body.WriteString("\n")
			}
			continue
		}
		if _, ok := heads[i]; ok {
			cur = &section{title: ln.Text, pageStart: ln.Page, pageEnd: ln.Page}
			sections = append(sections, cur)
			continue
		}
		if cur == nil {
			cur = &section{pageStart: ln.Page, pageEnd: ln.Page}
			sections = append(sections, cur)
		}
		cur.pageEnd = ln.Page
		cur.body.WriteString(ln.Text)
		cur.body.WriteString("\n")
	}
	return sections
}

func newChunk(sec *section, content string) Chunk {
	return Chunk{
		Section:   sec.title,
		PageStart: sec.pageStart,
		PageEnd:   sec.pageEnd,
		Content:   content,
		Tokens:    EstimateTokens(content),
	}
}

func withTitle(title, body string) string {
	if title == "" {
		return body
	}
	return title + "\n" + body
}

func splitLongSection(body string) []string {
	paras := reParagraphBreak.Split(body, -1)
	if len(paras) <= 1 {
		paras = strings.Split(body, "\n")
	}

	var pieces []string
	var cur strings.Builder
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if cur.Len() > 0 && EstimateTokens(cur.String())+EstimateTokens(p) > TargetTokens {
			pieces = append(pieces, strings.TrimSpace(cur.String()))
			tail := lastWords(cur.String(), OverlapWords)
			cur.Reset()
			cur.WriteString(tail)
			cur.WriteString("\n")
		}
		cur.WriteString(p)
		cur.WriteString("\n\n")
	}
	if strings.TrimSpace(cur.String()) != "" {
		pieces = append(pieces, strings.TrimSpace(cur.String()))
	}
	return pieces
}

func lastWords(s string, n int) string {
	words := strings.Fields(s)
	if len(words) <= n {
		return s
	}
	return strings.Join(words[len(words)-n:], " ")
}

func EstimateTokens(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return max(len(s)/CharsPerToken, len(strings.Fields(s)))
}
