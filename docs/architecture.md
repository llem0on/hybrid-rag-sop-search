# Architecture

## Components

```mermaid
flowchart LR
    C[Client] -->|HTTP| API[api]
    API --> IDX[indexer]
    API --> ENG[search engine]
    IDX --> CH[chunker]
    IDX --> FIG[figures]
    IDX --> EMB[embedding client]
    ENG --> BM[bm25]
    ENG --> EMB
    API --> ST[(store<br/>data/index.json)]
    EMB -->|/embeddings · /rerank| LLM[OpenAI-compatible endpoint]
    CH -->|pdftotext| P[poppler]
    FIG -->|pdftohtml · tesseract| P
```

| Package | Responsibility |
|---|---|
| `api` | HTTP routes, upload handling, engine rebuild after every change |
| `indexer` | Turns one PDF into chunks with vectors |
| `chunker` | Text extraction, boilerplate cleanup, heading detection, section chunks |
| `figures` | Image extraction, OCR, nearest heading and caption |
| `embedding` | Chat template, contextual header, HTTP client for embeddings and rerank |
| `bm25` | Tokenizer with Sastrawi stemming, BM25 index, query coverage |
| `search` | Lanes, fusion, relevance gate, rerank cut, snippet |
| `store` | Documents and chunks persisted as one JSON file, written atomically |

## Data model

```text
Document
  id, number, title, department, file_name, chunks, figures, created_at

Chunk
  id, document_id, index, section, page_start, page_end
  content, tokens
  figure_path        set only for figure chunks
  vector             float32, one per chunk
```

## Request lifecycle

### Upload

```mermaid
sequenceDiagram
    participant C as Client
    participant A as api
    participant I as indexer
    participant L as LLM endpoint
    participant S as store
    C->>A: POST /documents (PDF)
    A->>I: Build(document, path)
    I->>I: extract → clean → chunk
    I->>I: figures → OCR → section labels
    I->>L: embed chunks (batches of 16)
    L-->>I: vectors
    I-->>A: document + chunks
    A->>S: Add (atomic write)
    A->>A: rebuild search engine
    A-->>C: 201 document
```

### Search

```mermaid
sequenceDiagram
    participant C as Client
    participant E as engine
    participant L as LLM endpoint
    C->>E: POST /search
    E->>E: BM25 lane + metadata lane
    E->>L: embed query
    L-->>E: vector
    E->>E: cosine lane, RRF, relevance gate
    E->>L: rerank top candidates
    L-->>E: scores
    E->>E: cut, pin exact matches, snippets
    E-->>C: results + scores
```

## In-memory index

The engine holds every chunk vector and the BM25 index in memory. It is rebuilt from the store after each upload or delete and swapped under a lock, so searches never see a half-built index. Cosine over a few thousand 4096-dimensional vectors takes milliseconds; no vector database is needed at this size.

## Failure behaviour

| Failure | Behaviour |
|---|---|
| No `LLM_BASE_URL` | Keyword mode: BM25 and metadata lanes only |
| Query embedding fails or exceeds 15 s | That search runs in keyword mode |
| Reranker fails or exceeds `RERANK_TIMEOUT` | Fused results returned without the cut, `reranked: false` |
| Figure extraction or OCR fails | Document is indexed with its text chunks only |
| Document embedding fails | Upload returns 422; nothing is stored, extracted figures are removed |
