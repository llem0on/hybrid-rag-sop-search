package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/llem0on/hybrid-rag-sop-search/internal/model"
)

type snapshot struct {
	NextDocumentID int64            `json:"next_document_id"`
	NextChunkID    int64            `json:"next_chunk_id"`
	Documents      []model.Document `json:"documents"`
	Chunks         []model.Chunk    `json:"chunks"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	data snapshot
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "index.json"), data: snapshot{NextDocumentID: 1, NextChunkID: 1}}
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	return s, json.Unmarshal(raw, &s.data)
}

func (s *Store) Add(doc model.Document, chunks []model.Chunk) (model.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc.ID = s.data.NextDocumentID
	doc.CreatedAt = time.Now().UTC()
	s.data.NextDocumentID++
	for i := range chunks {
		chunks[i].ID = s.data.NextChunkID
		chunks[i].DocumentID = doc.ID
		s.data.NextChunkID++
	}
	s.data.Documents = append(s.data.Documents, doc)
	s.data.Chunks = append(s.data.Chunks, chunks...)
	return doc, s.save()
}

func (s *Store) Delete(id int64) (model.Document, []string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var removed model.Document
	found := false
	docs := s.data.Documents[:0]
	for _, d := range s.data.Documents {
		if d.ID == id {
			removed, found = d, true
			continue
		}
		docs = append(docs, d)
	}
	if !found {
		return removed, nil, false, nil
	}
	var figures []string
	chunks := s.data.Chunks[:0]
	for _, c := range s.data.Chunks {
		switch {
		case c.DocumentID != id:
			chunks = append(chunks, c)
		case c.FigurePath != "":
			figures = append(figures, c.FigurePath)
		}
	}
	s.data.Documents, s.data.Chunks = docs, chunks
	return removed, figures, true, s.save()
}

func (s *Store) Snapshot() ([]model.Document, []model.Chunk) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	docs := append([]model.Document(nil), s.data.Documents...)
	chunks := append([]model.Chunk(nil), s.data.Chunks...)
	return docs, chunks
}

func (s *Store) Documents() []model.Document {
	docs, _ := s.Snapshot()
	if docs == nil {
		docs = []model.Document{}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].ID < docs[j].ID })
	return docs
}

func (s *Store) save() error {
	raw, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
