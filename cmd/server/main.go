package main

import (
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/llem0on/hybrid-rag-sop-search/internal/api"
	"github.com/llem0on/hybrid-rag-sop-search/internal/config"
	"github.com/llem0on/hybrid-rag-sop-search/internal/embedding"
	"github.com/llem0on/hybrid-rag-sop-search/internal/indexer"
	"github.com/llem0on/hybrid-rag-sop-search/internal/search"
	"github.com/llem0on/hybrid-rag-sop-search/internal/store"
)

func main() {
	cfg := config.Load()

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}

	client := embedding.NewClient(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.EmbedModel, cfg.DocInstruction, cfg.QueryInstruction,
		cfg.RerankURL, cfg.RerankModel, cfg.EmbedTimeout, cfg.RerankTimeout)

	var embedder search.Embedder
	var docEmbedder indexer.DocumentEmbedder
	if client.EmbeddingEnabled() {
		embedder, docEmbedder = client, client
	} else {
		log.Print("LLM_BASE_URL not set: running in keyword-only mode")
	}
	var reranker search.Reranker
	if client.RerankEnabled() {
		reranker = client
	} else {
		log.Print("RERANK_MODEL not set: results are not reranked")
	}

	figuresDir := filepath.Join(cfg.DataDir, "figures")
	srv := &api.Server{
		Store:      st,
		SearchCfg:  cfg.Search,
		Embedder:   embedder,
		Reranker:   reranker,
		FilesDir:   filepath.Join(cfg.DataDir, "files"),
		FiguresDir: figuresDir,
		Indexer: &indexer.Indexer{
			Embedder:    docEmbedder,
			FiguresDir:  figuresDir,
			OCREnabled:  cfg.OCREnabled,
			OCRLanguage: cfg.OCRLanguage,
		},
	}

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("listening on %s (documents: %d)", cfg.Addr, len(st.Documents()))
	log.Fatal(httpServer.ListenAndServe())
}
