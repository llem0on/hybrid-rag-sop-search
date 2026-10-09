package search

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/llem0on/hybrid-rag-sop-search/internal/bm25"
	"github.com/llem0on/hybrid-rag-sop-search/internal/model"
)

const (
	RerankInstruction = "<Instruct>: Given an SOP search query, find the SOP section that answers it\n<Query>: "
	tieBreak          = 1e-4
	exactMatchBoost   = 1.0
	maxQueryChars     = 1000
	queryEmbedTimeout = 15 * time.Second
)

var ErrEmptyQuery = errors.New("query is required")

type Embedder interface {
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
}

type Reranker interface {
	Rerank(ctx context.Context, query string, docs []string) ([]float64, error)
}

type Scores struct {
	Final    float64  `json:"final"`
	Cosine   float64  `json:"cosine"`
	BM25     float64  `json:"bm25"`
	Coverage float64  `json:"coverage"`
	Metadata float64  `json:"metadata"`
	Rerank   *float64 `json:"rerank,omitempty"`
}

type Result struct {
	Document   model.Document `json:"document"`
	ChunkID    int64          `json:"chunk_id"`
	Section    string         `json:"section"`
	PageStart  int            `json:"page_start"`
	PageEnd    int            `json:"page_end"`
	Snippet    string         `json:"snippet"`
	FigurePath string         `json:"figure_path,omitempty"`
	Pass       string         `json:"pass"`
	Lanes      []string       `json:"lanes"`
	Scores     Scores         `json:"scores"`
	content    string
}

type Response struct {
	Query    string   `json:"query"`
	Mode     string   `json:"mode"`
	Reranked bool     `json:"reranked"`
	TookMS   int64    `json:"took_ms"`
	Results  []Result `json:"results"`
}

type Engine struct {
	cfg        Config
	docs       map[int64]model.Document
	chunks     map[int64]model.Chunk
	firstChunk map[int64]int64
	bm         *bm25.Index
	embedder   Embedder
	reranker   Reranker
}

type candidate struct {
	chunk    model.Chunk
	cosine   float64
	bm25     float64
	coverage float64
	meta     float64
	final    float64
	pass     string
	lanes    []string
}

func NewEngine(cfg Config, docs []model.Document, chunks []model.Chunk, embedder Embedder, reranker Reranker) *Engine {
	e := &Engine{
		cfg:        cfg,
		docs:       make(map[int64]model.Document, len(docs)),
		chunks:     make(map[int64]model.Chunk, len(chunks)),
		firstChunk: map[int64]int64{},
		embedder:   embedder,
		reranker:   reranker,
	}
	for _, d := range docs {
		e.docs[d.ID] = d
	}
	ids := make([]int64, 0, len(chunks))
	texts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		e.chunks[c.ID] = c
		if cur, ok := e.firstChunk[c.DocumentID]; !ok || c.Index < e.chunks[cur].Index {
			e.firstChunk[c.DocumentID] = c.ID
		}
		ids = append(ids, c.ID)
		texts = append(texts, c.Content)
	}
	e.bm = bm25.Build(ids, texts)
	return e
}

func Normalize(q string) string {
	return strings.ToLower(strings.Join(strings.Fields(q), " "))
}

func (e *Engine) Search(ctx context.Context, query string) (Response, error) {
	started := time.Now()
	norm := Normalize(query)
	if norm == "" {
		return Response{}, ErrEmptyQuery
	}
	if len(norm) > maxQueryChars {
		return Response{}, fmt.Errorf("query longer than %d characters", maxQueryChars)
	}

	if len(e.chunks) == 0 {
		return Response{Query: query, Mode: "empty_index", Results: []Result{}}, nil
	}

	kw, cov := e.bm.Search(norm, e.cfg.LaneTop)
	cosine, sem, mode := e.semanticLane(ctx, norm)
	meta := e.metadataLane(norm)

	cands := map[int64]*candidate{}
	add := func(id int64) {
		if _, ok := cands[id]; !ok {
			cands[id] = &candidate{chunk: e.chunks[id]}
		}
	}
	for id := range kw {
		add(id)
	}
	for id := range sem {
		add(id)
	}
	for _, c := range cands {
		if s := sectionScore(c.chunk.Section, norm); s > meta[c.chunk.DocumentID] {
			meta[c.chunk.DocumentID] = s
		}
	}
	represented := map[int64]bool{}
	for _, c := range cands {
		represented[c.chunk.DocumentID] = true
	}
	for docID := range meta {
		if first, ok := e.firstChunk[docID]; ok && !represented[docID] {
			add(first)
		}
	}

	results := e.fuse(cands, kw, cov, sem, cosine, meta, mode)
	resp := Response{Query: query, Mode: mode, Results: results}
	if e.reranker != nil && len(results) > 0 {
		if cut, err := e.rerank(ctx, norm, results); err == nil {
			resp.Results, resp.Reranked = cut, true
		}
	}
	resp.Results = pinExactMatches(resp.Results)
	for i := range resp.Results {
		resp.Results[i].Snippet = snippet(resp.Results[i].content, norm)
	}
	resp.TookMS = time.Since(started).Milliseconds()
	return resp, nil
}

func (e *Engine) semanticLane(ctx context.Context, norm string) (all, top map[int64]float64, mode string) {
	all, top = map[int64]float64{}, map[int64]float64{}
	if e.embedder == nil {
		return all, top, "keyword_only"
	}
	ectx, cancel := context.WithTimeout(ctx, queryEmbedTimeout)
	defer cancel()
	qv, err := e.embedder.EmbedQuery(ectx, norm)
	if err != nil {
		return all, top, "keyword_only"
	}
	for id, c := range e.chunks {
		if len(c.Vector) == len(qv) {
			all[id] = cosineSimilarity(qv, c.Vector)
		}
	}
	return all, bm25.TopN(all, e.cfg.LaneTop), "hybrid"
}

func (e *Engine) metadataLane(norm string) map[int64]float64 {
	out := map[int64]float64{}
	for id, d := range e.docs {
		if s := metadataScore(d, norm); s > 0 {
			out[id] = s
		}
	}
	return out
}

func (e *Engine) fuse(cands map[int64]*candidate, kw, cov, sem, cosine, meta map[int64]float64, mode string) []Result {
	kwRank, semRank, metaRank := rankOf(kw), rankOf(sem), rankOf(meta)
	ws, wk := laneWeights(e.cfg.WSemantic, e.cfg.WKeyword)
	k := e.cfg.RRFK

	best := map[int64]*candidate{}
	for id, c := range cands {
		docID := c.chunk.DocumentID
		c.cosine, c.bm25, c.coverage, c.meta = cosine[id], kw[id], cov[id], meta[docID]
		if r, ok := semRank[id]; ok {
			c.final += ws / (k + float64(r))
			c.lanes = append(c.lanes, "semantic")
		}
		if r, ok := kwRank[id]; ok {
			c.final += wk / (k + float64(r))
			c.lanes = append(c.lanes, "keyword")
		}
		if r, ok := metaRank[docID]; ok {
			c.final += 1 / (k + float64(r))
			c.lanes = append(c.lanes, "metadata")
		}
		c.final += c.cosine * tieBreak
		if c.meta >= metaExactTitle {
			c.final += exactMatchBoost
		}
		c.pass = e.passReason(c, mode)
		if c.pass == "" {
			continue
		}
		if cur, ok := best[docID]; !ok || c.final > cur.final {
			best[docID] = c
		}
	}

	out := make([]Result, 0, len(best))
	for docID, c := range best {
		doc, ok := e.docs[docID]
		if !ok {
			continue
		}
		out = append(out, Result{
			Document:   doc,
			ChunkID:    c.chunk.ID,
			Section:    c.chunk.Section,
			PageStart:  c.chunk.PageStart,
			PageEnd:    c.chunk.PageEnd,
			FigurePath: c.chunk.FigurePath,
			Pass:       c.pass,
			Lanes:      c.lanes,
			Scores:     Scores{Final: round4(c.final), Cosine: round4(c.cosine), BM25: round4(c.bm25), Coverage: round4(c.coverage), Metadata: c.meta},
			content:    c.chunk.Content,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Scores.Final > out[j].Scores.Final })
	if len(out) > e.cfg.Candidates {
		out = out[:e.cfg.Candidates]
	}
	return out
}

func (e *Engine) passReason(c *candidate, mode string) string {
	switch {
	case c.meta >= metaTitleContains:
		return "metadata"
	case c.bm25 >= e.cfg.BM25Min && c.coverage >= e.cfg.BM25MinCoverage:
		return "bm25_strong"
	case mode == "hybrid" && c.cosine >= e.cfg.MinSemantic:
		return "semantic"
	}
	return ""
}

func (e *Engine) rerank(ctx context.Context, norm string, results []Result) ([]Result, error) {
	docs := make([]string, len(results))
	for i, r := range results {
		docs[i] = RerankDocument(r.Document.Title, r.Section, r.content, e.cfg.RerankContentChars)
	}
	scores, err := e.reranker.Rerank(ctx, RerankInstruction+norm, docs)
	if err != nil {
		return nil, err
	}
	out := make([]Result, 0, e.cfg.MaxResults)
	for i, r := range results {
		s := round4(scores[i])
		r.Scores.Rerank = &s
		if len(out) < e.cfg.MaxResults && e.keep(i, r, scores[i]) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (e *Engine) keep(rank int, r Result, rerank float64) bool {
	if r.Scores.Metadata >= metaTitleContains || rerank >= e.cfg.RerankMin {
		return true
	}
	return rank == 0 && (rerank >= e.cfg.RerankTopMin || r.Scores.Cosine >= e.cfg.RerankTopCos)
}

func RerankDocument(title, section, content string, maxChars int) string {
	r := []rune(content)
	if len(r) > maxChars {
		r = r[:maxChars]
	}
	return fmt.Sprintf("Document: %s\nSection: %s\n\n%s", title, section, string(r))
}

func pinExactMatches(results []Result) []Result {
	pinned := make([]Result, 0, len(results))
	rest := make([]Result, 0, len(results))
	for _, r := range results {
		if r.Scores.Metadata >= metaExactTitle {
			pinned = append(pinned, r)
		} else {
			rest = append(rest, r)
		}
	}
	return append(pinned, rest...)
}

func cosineSimilarity(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func round4(v float64) float64 {
	return math.Round(v*1e4) / 1e4
}
