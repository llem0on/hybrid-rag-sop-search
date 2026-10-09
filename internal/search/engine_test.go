package search

import (
	"context"
	"errors"
	"testing"

	"github.com/llem0on/hybrid-rag-sop-search/internal/model"
)

type fakeEmbedder map[string][]float32

func (f fakeEmbedder) EmbedQuery(_ context.Context, q string) ([]float32, error) {
	if v, ok := f[q]; ok {
		return v, nil
	}
	return []float32{0, 0, 1}, nil
}

type fakeReranker struct {
	scores []float64
	err    error
}

func (f fakeReranker) Rerank(_ context.Context, _ string, docs []string) ([]float64, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.scores[:len(docs)], nil
}

func fixture() ([]model.Document, []model.Chunk) {
	docs := []model.Document{
		{ID: 1, Number: "DOC 01.001.01", Title: "Data Backup and Restore"},
		{ID: 2, Number: "DOC 01.002.01", Title: "Password Policy"},
		{ID: 3, Number: "DOC 01.003.01", Title: "Incident Escalation"},
	}
	chunks := []model.Chunk{
		{ID: 10, DocumentID: 1, Section: "3. Procedure", Content: "3. Procedure\nRun a full backup of every database each night and verify the restore weekly.", Vector: []float32{1, 0, 0}},
		{ID: 20, DocumentID: 2, Section: "2. Rules", Content: "2. Rules\nPasswords must have twelve characters and rotate every ninety days.", Vector: []float32{0, 1, 0}},
		{ID: 30, DocumentID: 3, Section: "4. Flow", Content: "4. Flow\nEscalate unresolved incidents to the on-call engineer after thirty minutes.", Vector: []float32{0.6, 0.8, 0}},
	}
	return docs, chunks
}

func TestSemanticPassAndRerankCut(t *testing.T) {
	docs, chunks := fixture()
	emb := fakeEmbedder{"how often is the database backed up": {0.9, 0.1, 0}}
	e := NewEngine(DefaultConfig(), docs, chunks, emb, fakeReranker{scores: []float64{0.82, 0.31, 0.2}})

	resp, err := e.Search(context.Background(), "How often is the database backed up")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Reranked || len(resp.Results) != 1 || resp.Results[0].Document.ID != 1 {
		t.Fatalf("want only backup doc, got %+v", resp.Results)
	}
}

func TestRankOneNeedsRerankOrCosineFloor(t *testing.T) {
	docs, chunks := fixture()
	emb := fakeEmbedder{"weather tomorrow": {1, 0, 1.25}}
	e := NewEngine(DefaultConfig(), docs, chunks, emb, fakeReranker{scores: []float64{0.05, 0.02, 0.01}})

	resp, _ := e.Search(context.Background(), "weather tomorrow")
	if len(resp.Results) != 0 {
		t.Fatalf("rank 1 with cosine < 0.65 and rerank < 0.15 must be cut, got %+v", resp.Results)
	}
}

func TestExactNumberIsPinned(t *testing.T) {
	docs, chunks := fixture()
	e := NewEngine(DefaultConfig(), docs, chunks, nil, nil)

	resp, _ := e.Search(context.Background(), "doc-01-002-01")
	if len(resp.Results) == 0 || resp.Results[0].Document.ID != 2 || resp.Results[0].Pass != "metadata" {
		t.Fatalf("number match not pinned: %+v", resp.Results)
	}
	if resp.Mode != "keyword_only" {
		t.Fatalf("want keyword_only without embedder, got %s", resp.Mode)
	}
}

func TestRerankFailureReturnsUncutResults(t *testing.T) {
	docs, chunks := fixture()
	emb := fakeEmbedder{"backup": {1, 0, 0}}
	e := NewEngine(DefaultConfig(), docs, chunks, emb, fakeReranker{err: errors.New("timeout")})

	resp, _ := e.Search(context.Background(), "backup")
	if resp.Reranked || len(resp.Results) == 0 {
		t.Fatalf("want uncut results on rerank failure, got %+v", resp)
	}
}

func TestMetadataCompactNumber(t *testing.T) {
	d := model.Document{Number: "DOC 12.003.01", Title: "Access Control"}
	for _, q := range []string{"doc 12.003.01", "doc12.003.01", "doc-12-003-01"} {
		if metadataScore(d, q) != metaExactNumber {
			t.Fatalf("%q should match number exactly", q)
		}
	}
	if metadataScore(d, "access") != metaTitleContains {
		t.Fatal("title substring should score 0.80")
	}
}
