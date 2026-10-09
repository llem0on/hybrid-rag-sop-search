package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/llem0on/hybrid-rag-sop-search/internal/indexer"
	"github.com/llem0on/hybrid-rag-sop-search/internal/model"
	"github.com/llem0on/hybrid-rag-sop-search/internal/search"
	"github.com/llem0on/hybrid-rag-sop-search/internal/store"
)

const maxUploadBytes = 50 << 20

var reUnsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type Server struct {
	Store      *store.Store
	Indexer    *indexer.Indexer
	SearchCfg  search.Config
	Embedder   search.Embedder
	Reranker   search.Reranker
	FilesDir   string
	FiguresDir string

	mu     sync.RWMutex
	engine *search.Engine
}

func (s *Server) Routes() http.Handler {
	s.Rebuild()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /documents", s.listDocuments)
	mux.HandleFunc("POST /documents", s.uploadDocument)
	mux.HandleFunc("DELETE /documents/{id}", s.deleteDocument)
	mux.HandleFunc("POST /search", s.search)
	mux.HandleFunc("GET /figures/{name}", s.figure)
	return logRequests(mux)
}

func (s *Server) Rebuild() {
	docs, chunks := s.Store.Snapshot()
	engine := search.NewEngine(s.SearchCfg, docs, chunks, s.Embedder, s.Reranker)
	s.mu.Lock()
	s.engine = engine
	s.mu.Unlock()
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"documents": len(s.Store.Documents()),
		"embedding": s.Embedder != nil,
		"reranker":  s.Reranker != nil,
	})
}

func (s *Server) listDocuments(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Store.Documents())
}

func (s *Server) uploadDocument(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "multipart field 'file' (PDF) is required")
		return
	}
	defer file.Close()
	if !strings.EqualFold(filepath.Ext(header.Filename), ".pdf") {
		writeError(w, http.StatusBadRequest, "only PDF files are supported")
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}
	name := fmt.Sprintf("%d-%s", time.Now().UnixNano(), reUnsafeName.ReplaceAllString(header.Filename, "_"))
	path := filepath.Join(s.FilesDir, name)
	if err := saveUpload(file, path); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	doc := model.Document{
		Number:     strings.TrimSpace(r.FormValue("number")),
		Title:      title,
		Department: strings.TrimSpace(r.FormValue("department")),
		FileName:   name,
	}
	doc, chunks, err := s.Indexer.Build(r.Context(), doc, path)
	if err != nil {
		os.Remove(path)
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	doc, err = s.Store.Add(doc, chunks)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Rebuild()
	writeJSON(w, http.StatusCreated, doc)
}

func (s *Server) deleteDocument(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	doc, figures, found, err := s.Store.Delete(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	os.Remove(filepath.Join(s.FilesDir, doc.FileName))
	for _, f := range figures {
		os.Remove(filepath.Join(s.FiguresDir, f))
	}
	s.Rebuild()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body must be JSON: {\"query\": \"...\"}")
		return
	}
	s.mu.RLock()
	engine := s.engine
	s.mu.RUnlock()

	resp, err := engine.Search(r.Context(), req.Query)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	log.Printf("search query=%q mode=%s reranked=%v results=%d took_ms=%d", req.Query, resp.Mode, resp.Reranked, len(resp.Results), resp.TookMS)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) figure(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.PathValue("name"))
	path := filepath.Join(s.FiguresDir, name)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusNotFound, "figure not found")
		return
	}
	http.ServeFile(w, r, path)
}

func saveUpload(src io.Reader, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
