package bm25

import (
	"math"
	"regexp"
	"sort"
	"strings"

	sastrawi "github.com/RadhiFadlillah/go-sastrawi"
)

const (
	k1 = 1.2
	b  = 0.75
)

var (
	tokenRe = regexp.MustCompile(`[a-z0-9]+(?:\.[a-z0-9]+)*`)
	stemmer = sastrawi.NewStemmer(sastrawi.DefaultDictionary())

	Stopwords = map[string]bool{
		"yang": true, "dan": true, "di": true, "ke": true, "dari": true, "untuk": true,
		"pada": true, "dengan": true, "atau": true, "ini": true, "itu": true, "adalah": true,
		"akan": true, "sebagai": true, "oleh": true, "dalam": true, "tidak": true, "juga": true,
		"agar": true, "serta": true, "bagi": true, "saat": true, "telah": true, "sudah": true,
		"ada": true, "apa": true, "cara": true, "gimana": true, "bagaimana": true, "bener": true,
		"benar": true, "the": true, "of": true, "and": true, "for": true, "in": true, "is": true,
	}
)

func Stem(w string) string {
	if strings.ContainsAny(w, "0123456789.") {
		return w
	}
	return stemmer.Stem(w)
}

func Tokens(text string) []string {
	raw := tokenRe.FindAllString(strings.ToLower(text), -1)
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		if len(t) < 2 || Stopwords[t] {
			continue
		}
		out = append(out, Stem(t))
	}
	return out
}

type Index struct {
	ids  []int64
	tf   []map[string]int
	lens []int
	avg  float64
	idf  map[string]float64
}

func Build(ids []int64, texts []string) *Index {
	ix := &Index{ids: ids, tf: make([]map[string]int, len(texts)), lens: make([]int, len(texts)), idf: map[string]float64{}}
	df := map[string]int{}
	total := 0
	for i, text := range texts {
		toks := Tokens(text)
		tf := map[string]int{}
		for _, w := range toks {
			tf[w]++
		}
		for w := range tf {
			df[w]++
		}
		ix.tf[i], ix.lens[i] = tf, len(toks)
		total += len(toks)
	}
	n := float64(len(texts))
	if n > 0 {
		ix.avg = float64(total) / n
	}
	for w, d := range df {
		ix.idf[w] = math.Log(1 + (n-float64(d)+0.5)/(float64(d)+0.5))
	}
	return ix
}

func (ix *Index) Search(query string, limit int) (scores, coverage map[int64]float64) {
	scores, coverage = map[int64]float64{}, map[int64]float64{}
	if ix == nil || ix.avg == 0 {
		return scores, coverage
	}
	terms := map[string]bool{}
	for _, w := range Tokens(query) {
		terms[w] = true
	}
	if len(terms) == 0 {
		return scores, coverage
	}
	for i, tf := range ix.tf {
		norm := k1 * (1 - b + b*float64(ix.lens[i])/ix.avg)
		s, matched := 0.0, 0
		for w := range terms {
			if f := tf[w]; f > 0 {
				s += ix.idf[w] * float64(f) * (k1 + 1) / (float64(f) + norm)
				matched++
			}
		}
		if s > 0 {
			scores[ix.ids[i]] = s
			coverage[ix.ids[i]] = float64(matched) / float64(len(terms))
		}
	}
	return TopN(scores, limit), coverage
}

func TopN(scores map[int64]float64, n int) map[int64]float64 {
	if n <= 0 || len(scores) <= n {
		return scores
	}
	ids := make([]int64, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return scores[ids[i]] > scores[ids[j]] })
	out := make(map[int64]float64, n)
	for _, id := range ids[:n] {
		out[id] = scores[id]
	}
	return out
}
