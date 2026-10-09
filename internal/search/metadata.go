package search

import (
	"sort"
	"strings"
	"unicode"

	"github.com/llem0on/hybrid-rag-sop-search/internal/model"
)

const (
	metaExactNumber   = 1.00
	metaExactTitle    = 0.95
	metaTitleContains = 0.80
	metaSectionMatch  = 0.65
	metaDepartment    = 0.40
)

func metadataScore(doc model.Document, norm string) float64 {
	if norm == "" {
		return 0
	}
	number := compact(doc.Number)
	query := compact(norm)
	title := strings.ToLower(strings.TrimSpace(doc.Title))
	dept := strings.ToLower(strings.TrimSpace(doc.Department))

	switch {
	case number != "" && number == query:
		return metaExactNumber
	case title != "" && title == norm:
		return metaExactTitle
	case title != "" && strings.Contains(title, norm):
		return metaTitleContains
	case number != "" && query != "" && strings.Contains(number, query):
		return metaTitleContains
	case dept != "" && dept == norm:
		return metaDepartment
	}
	return 0
}

func sectionScore(section, norm string) float64 {
	section = strings.ToLower(strings.TrimSpace(section))
	if section != "" && norm != "" && strings.Contains(section, norm) {
		return metaSectionMatch
	}
	return 0
}

func compact(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}

func rankOf(scores map[int64]float64) map[int64]int {
	ids := make([]int64, 0, len(scores))
	for id, s := range scores {
		if s > 0 {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if scores[ids[i]] != scores[ids[j]] {
			return scores[ids[i]] > scores[ids[j]]
		}
		return ids[i] < ids[j]
	})
	out := make(map[int64]int, len(ids))
	for i, id := range ids {
		out[id] = i + 1
	}
	return out
}

func laneWeights(wSemantic, wKeyword float64) (float64, float64) {
	sum := wSemantic + wKeyword
	if wSemantic <= 0 || wKeyword <= 0 {
		return 1, 1
	}
	return 2 * wSemantic / sum, 2 * wKeyword / sum
}
