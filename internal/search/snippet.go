package search

import (
	"regexp"
	"strings"

	"github.com/llem0on/hybrid-rag-sop-search/internal/bm25"
)

const snippetMaxRunes = 300

var reSentence = regexp.MustCompile(`[^.!?\n]+[.!?]?`)

func snippet(content, norm string) string {
	terms := map[string]bool{}
	for _, t := range strings.Fields(norm) {
		t = strings.Trim(t, ".,;:()[]\"'")
		if len(t) > 2 && !bm25.Stopwords[t] {
			terms[t] = true
			terms[bm25.Stem(t)] = true
		}
	}

	best, bestHits := "", 0
	for _, s := range reSentence.FindAllString(content, -1) {
		s = strings.TrimSpace(s)
		n := len([]rune(s))
		if n < 25 || n > 320 {
			continue
		}
		lower := strings.ToLower(s)
		hits := 0
		for t := range terms {
			if strings.Contains(lower, t) {
				hits++
			}
		}
		if hits > bestHits {
			best, bestHits = s, hits
		}
	}
	if best != "" {
		return best
	}
	flat := strings.Join(strings.Fields(content), " ")
	r := []rune(flat)
	if len(r) > snippetMaxRunes {
		return string(r[:snippetMaxRunes]) + "..."
	}
	return flat
}
