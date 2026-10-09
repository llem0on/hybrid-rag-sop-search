package indexer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/llem0on/hybrid-rag-sop-search/internal/chunker"
	"github.com/llem0on/hybrid-rag-sop-search/internal/embedding"
	"github.com/llem0on/hybrid-rag-sop-search/internal/figures"
	"github.com/llem0on/hybrid-rag-sop-search/internal/model"
)

type DocumentEmbedder interface {
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
}

type Indexer struct {
	Embedder    DocumentEmbedder
	FiguresDir  string
	OCREnabled  bool
	OCRLanguage string
}

func (ix *Indexer) Build(ctx context.Context, doc model.Document, pdfPath string) (model.Document, []model.Chunk, error) {
	pages, err := chunker.ExtractPages(ctx, pdfPath)
	if err != nil {
		return doc, nil, err
	}
	pieces := chunker.ChunkPages(chunker.CleanPages(pages))
	if len(pieces) == 0 {
		return doc, nil, fmt.Errorf("no text found in %s", filepath.Base(pdfPath))
	}

	purpose := ""
	for _, p := range pieces {
		if embedding.IsPurposeSection(p.Section) {
			if purpose = embedding.PurposeSentence(p.Content); purpose != "" {
				break
			}
		}
	}

	chunks := make([]model.Chunk, 0, len(pieces))
	texts := make([]string, 0, len(pieces))
	for _, p := range pieces {
		chunks = append(chunks, model.Chunk{
			Index:     len(chunks),
			Section:   p.Section,
			PageStart: p.PageStart,
			PageEnd:   p.PageEnd,
			Content:   p.Content,
			Tokens:    p.Tokens,
		})
		texts = append(texts, embedding.DocumentText(doc.Title, p.Section, purpose, p.Content))
	}

	if ix.OCREnabled {
		prefix := strconv.FormatInt(time.Now().UnixNano(), 36)
		figs, err := figures.Extract(ctx, pdfPath, ix.FiguresDir, prefix, ix.OCRLanguage)
		if err != nil {
			log.Printf("figures skipped for %s: %v", filepath.Base(pdfPath), err)
		}
		for _, f := range figs {
			chunks = append(chunks, model.Chunk{
				Index:      len(chunks),
				Section:    f.Section,
				PageStart:  f.Page,
				PageEnd:    f.Page,
				Content:    f.Text,
				Tokens:     chunker.EstimateTokens(f.Text),
				FigurePath: f.Path,
			})
			texts = append(texts, embedding.FigureText(doc.Title, f.Section, f.Text))
			doc.Figures++
		}
	}

	if ix.Embedder != nil {
		vectors, err := ix.Embedder.EmbedDocuments(ctx, texts)
		if err != nil {
			for _, c := range chunks {
				if c.FigurePath != "" {
					os.Remove(filepath.Join(ix.FiguresDir, c.FigurePath))
				}
			}
			return doc, nil, err
		}
		for i := range chunks {
			chunks[i].Vector = vectors[i]
		}
	}
	doc.Chunks = len(chunks)
	return doc, chunks, nil
}
