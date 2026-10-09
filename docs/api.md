# API

Base URL: `http://localhost:8080`

## `POST /documents`

Upload and index one PDF.

| Field | Type | Required | Description |
|---|---|---|---|
| `file` | file | yes | PDF, max 50 MB |
| `title` | text | no | Defaults to the file name |
| `number` | text | no | Document number, matched exactly by the metadata lane |
| `department` | text | no | Owning department |

```bash
curl -F file=@backup.pdf -F title="Data Backup and Restore" -F number="DOC 01.001.01" \
  localhost:8080/documents
```

```json
{ "id": 1, "number": "DOC 01.001.01", "title": "Data Backup and Restore", "department": "", "file_name": "1760000000-backup.pdf", "chunks": 14, "figures": 2, "created_at": "2026-10-09T10:00:00Z" }
```

| Status | Meaning |
|---|---|
| `201` | Indexed |
| `400` | Missing file or not a PDF |
| `422` | No text found, or the embedding endpoint failed |

## `GET /documents`

```json
[{ "id": 1, "number": "DOC 01.001.01", "title": "Data Backup and Restore", "chunks": 14, "figures": 2 }]
```

## `DELETE /documents/{id}`

Removes the document, its chunks, its stored PDF and its figure images. Returns `204`, or `404` when the id is unknown.

## `POST /search`

```bash
curl -s localhost:8080/search -d '{"query": "how often is the database backed up"}'
```

| Field | Description |
|---|---|
| `mode` | `hybrid`, `keyword_only` (no embedding available) or `empty_index` |
| `reranked` | `false` when no reranker is configured or the call failed |
| `results[].pass` | `metadata`, `bm25_strong` or `semantic` |
| `results[].lanes` | Lanes that found the chunk |
| `results[].scores` | `final`, `cosine`, `bm25`, `coverage`, `metadata`, `rerank` |
| `results[].figure_path` | Present for figure chunks; fetch with `/figures/{name}` |

## `GET /figures/{name}`

Returns the PNG of a figure chunk.

## `GET /health`

```json
{ "status": "ok", "documents": 12, "embedding": true, "reranker": true }
```
